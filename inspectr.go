package main

import (
	"bytes"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	jira "github.com/andygrunwald/go-jira"
	"github.com/golang/glog"
)

//AvailableImageData type
type AvailableImageData interface {
	tag() string
}

//DockerTag type representing the json schema of docker registry versions page
//e.g. https://registry.hub.docker.com/v1/repositories/eversc/inspectr/tags
type DockerTag struct {
	Layer string `json:"layer"`
	Name  string `json:"name"`
}

//QuayTag type
type QuayTag struct {
	Name string
}

//GcrTag type
type GcrTag struct {
	Name string
}

//SlackMsg type
type SlackMsg struct {
	Text     string `json:"text"`
	Username string `json:"username"`
}

//InspectrResult type
type InspectrResult struct {
	Name      string
	Namespace string
	Quantity  int64
	Upgrades  []string
	Version   string
}

func main() {
	flag.Parse()
	glog.Info("hello inspectr")
	registeredImages := make(map[string][]string)
	glog.Info("initialized local image registry cache")

	slackWebhookKey := "INSPECTR_SLACK_WEBHOOK_ID"
	jiraURLKey := "INSPECTR_JIRA_URL"
	jiraParamKey := "INSPECTR_JIRA_PARAMS"

	webhookID := os.Getenv(slackWebhookKey)
	jiraURL := os.Getenv(jiraURLKey)
	jiraParams := os.Getenv(jiraParamKey)

	glog.Info("picked up env vars")
	glog.Info("about to enter life-of-pod loop")
	for {
		time.Sleep(time.Duration(invokeInspectrProcess(&registeredImages, webhookID, jiraURL, jiraParams)) * time.Second)
	}
}

//invokeInspectrProcess attempts to run through as much of the 'process' as it can. At appropriate points it may
// abort if it sees an error. If this happens, the error is logged, and a default/long time is returned as the sleep
// value.
// if everything goes okay, the sleep value returned is either pretty small (as inspectr should be quick to detect
// any 'unregistered' images, or a bit longer if current time is withinAlertWindow
func invokeInspectrProcess(registeredImages *map[string][]string, webhookID, jiraURL, jiraParamString string) (sleep int) {
	sleep = 300
	var k8sJSONData *Data
	var err error
	k8sJSONData, err = jsonData()
	if err == nil {
		withinAlertWindow := withinAlertWindow()
		var upgradeMap map[string][]InspectrResult
		upgradeMap, err = upgradesMap(imageToResultsMap(k8sJSONData))
		if err == nil {
			upgradeMap = filterUpgradesMap(upgradeMap, *registeredImages, withinAlertWindow)
			glog.Info("filterUpgradesMap()")
			augmentInternalImageRegistry(upgradeMap, *registeredImages, withinAlertWindow)
			glog.Info("augmentInternalImageRegistry()")
			outputResults(upgradeMap, webhookID, withinAlertWindow)
			reportResults(upgradeMap, jiraURL, jiraParamString, webhookID)
			sleep = sleepTime(withinAlertWindow)
		}
	}
	if err != nil {
		glog.Error(err, ", going to sleep for "+strconv.Itoa(sleep)+" seconds")
	}
	return
}

//jsonData returns a Data struct based on what k8s master returns, and an error
func jsonData() (jsonData *Data, err error) {
	bodyReader, err := bodyFromMaster()
	if err == nil {
		jsonData, err = decodeData(bodyReader)
		bodyReader.Close()
	}
	return jsonData, err
}

//sleepTime returns an int of the number of seconds to go to sleep for. Sleep is needed so the process isn't
// constantly running, and doesn't run more than once in the alert window
func sleepTime(withinAlertWindow bool) (sleepTime int) {
	if withinAlertWindow {
		sleepTime = 360
	} else {
		sleepTime = 60
	}
	return
}

//withinAlertWindow returns a bool indicating whether current time is within the scheduled daily/weekly alert window
func withinAlertWindow() (withinAlertWindow bool) {
	//TODO: get a weekday value (from env var?)
	weekday := -1
	now := time.Now()
	preferredAlertTime := time.Date(int(now.Year()), now.Month(), int(now.Day()),
		11, 30, 0, 0, now.Location())
	windowSize := 300
	windowEnd := preferredAlertTime.Add(time.Duration(windowSize) * time.Second)
	if weekday == -1 || int(now.Weekday()) == weekday {
		withinAlertWindow = now.After(preferredAlertTime) && now.Before(windowEnd)
	}
	return
}

//filterUpgradesMap returns a map which is based on the one specified, but has any already registered upgrade
// opportunities removed. If we're withinAlertWindow, no filtering happens
func filterUpgradesMap(upgradesMap map[string][]InspectrResult, registeredImages map[string][]string,
	withinAlertWindow bool) (filteredMap map[string][]InspectrResult) {

	if withinAlertWindow {
		filteredMap = upgradesMap
	} else {
		filteredMap = make(map[string][]InspectrResult)
		for k, v := range upgradesMap {
			registeredResults, ok := registeredImages[k]
			if ok {
				filteredSlice := make([]InspectrResult, 0)
				for _, result := range v {
					if !sliceContainsResult(registeredResults, result) {
						filteredSlice = append(filteredSlice, result)
					}
				}
				if len(filteredSlice) > 0 {
					filteredMap[k] = filteredSlice
				}
			} else {
				filteredMap[k] = v
			}
		}
	}
	return
}

//sliceContainsResult returns a bool indicating whether the specified InspectrResult is 'registered' in the string slice
func sliceContainsResult(registeredResults []string, result InspectrResult) (containsResult bool) {
	for _, registeredResult := range registeredResults {
		splitStrings := strings.Split(registeredResult, "|")
		if splitStrings[0] == result.Version && splitStrings[1] == result.Namespace {
			containsResult = true
			break
		}
	}
	return
}

//augmentInternalImageRegistry will, based on whether we're withinAlertWindow, replace the registeredImageMap or
// augment it with any InspectrResults that are missing, respectively
func augmentInternalImageRegistry(upgradesMap map[string][]InspectrResult, registeredImageMap map[string][]string,
	withinAlertWindow bool) map[string][]string {

	if withinAlertWindow {
		registeredImageMap = registeredImages(upgradesMap)
	} else {
		for k, v := range upgradesMap {
			registeredResults, ok := registeredImageMap[k]
			if ok {
				for _, result := range v {
					if !sliceContainsResult(registeredResults, result) {
						registeredResults = append(registeredResults, registeredImageString(result))
						registeredImageMap[k] = registeredResults
					}
				}
			} else {
				registeredImageMap[k] = stringSliceFromResultSlice(v)
			}
		}
	}
	return registeredImageMap
}

//registeredImages returns a string<-->string map that reflects the string<-->[]InspectrResult map
func registeredImages(upgradesMap map[string][]InspectrResult) (registeredImages map[string][]string) {
	for k, v := range upgradesMap {
		registeredImageSlice := make([]string, 0)
		for _, upgradeResult := range v {
			registeredImageSlice = append(registeredImageSlice, registeredImageString(upgradeResult))
		}
		if registeredImages == nil {
			registeredImages = make(map[string][]string, 0)
		}
		registeredImages[k] = registeredImageSlice
	}
	return
}

//stringSliceFromResultSlice returns a string slice that represents the specified []InspectrResult
func stringSliceFromResultSlice(resultSlice []InspectrResult) (registeredResults []string) {
	for _, result := range resultSlice {
		registeredResults = append(registeredResults, registeredImageString(result))
	}
	return
}

//registeredImageString returns a string consisting of [result.Version]|[result.Namespace], helpful for the
// image registry cache
func registeredImageString(result InspectrResult) (resultString string) {
	resultString = result.Version + "|" + result.Namespace
	return
}

//upgradesMap returns a string <--> []InspectrResult only for those images with upgrades available
func upgradesMap(imageToResultsMap map[string][]InspectrResult) (upgradesMap map[string][]InspectrResult, err error) {
	upgradesMap = make(map[string][]InspectrResult)
	for k, v := range imageToResultsMap {
		imageString := strings.Split(k, ":")[1]
		var availImages []AvailableImageData
		switch {
		case strings.Contains(imageString, "quay.io"):
			availImages, err = quayTagSlice(imageString)
		case strings.Contains(imageString, "gcr.io"):
			availImages, err = gcrTagSlice(imageString)
		default:
			availImages, err = dockerTagSlice(imageString)
		}
		if err == nil {
			upgradesResults := make([]InspectrResult, 0)
			for _, result := range v {
				for _, upgradeVersion := range upgradeCandidateSlice(result.Version, []AvailableImageData(availImages)) {
					result.Upgrades = append(result.Upgrades, upgradeVersion.tag())
				}
				if len(result.Upgrades) > 0 {
					upgradesResults = append(upgradesResults, result)
				}
			}
			if len(upgradesResults) > 0 {
				upgradesMap[k] = upgradesResults
			}
		}
	}
	return
}

//Dockertag implementation of AvailableImageData
func (dockerTag DockerTag) tag() string {
	return dockerTag.Name
}

//QuayTag implementation of AvailableImageData
func (quayTag QuayTag) tag() string {
	return quayTag.Name
}

//GcrTag implementaton of AvailableImageData
func (gcrTag GcrTag) tag() string {
	return gcrTag.Name
}

//upgradeCandidateSlice returns a slice of AvailableImageData types that are deemed to be upgrades to the version
//specified
func upgradeCandidateSlice(version string, availImagesData []AvailableImageData) (upgradeCandidates []AvailableImageData) {
	versionPrefix := versionPrefix(version)
	suffix := suffix(version)
	versionNumerics := versionNumerics(version, suffix, versionPrefix)
	for _, availImageData := range availImagesData {
		if upgradeable(versionNumerics, availImageData.tag(), suffix, versionPrefix) {
			upgradeCandidates = append(upgradeCandidates, availImageData)
		}
	}
	return
}

//versionPrefix returns a bool indicating whether the specified string is prefixed with "v"
func versionPrefix(version string) (versionPrefix bool) {
	versionPrefix = strings.HasPrefix(version, "v")
	return
}

//suffix returns the suffix of the string, with the suffix being anything after (and including) a hyphen at the tail
// end of the string
func suffix(version string) (suffix string) {
	suffixStrings := strings.Split(version, "-")
	if len(suffixStrings) > 1 {
		suffix = suffixStrings[1]
	}
	return
}

//versionNumerics returns a slice of the numerical values in the specified version string
func versionNumerics(version, suffix string, versionPrefix bool) (versionNumerics []int) {
	versionNumeric := version
	if versionPrefix {
		versionNumeric = versionNumeric[1 : len(versionNumeric)-1]
	}
	versionNumeric = strings.Trim(versionNumeric, suffix)
	versionStringSlice := strings.Split(versionNumeric, ".")
	for _, versionString := range versionStringSlice {
		if numeric, err := strconv.Atoi(versionString); err == nil {
			versionNumerics = append(versionNumerics, numeric)
		}
	}
	return
}

//upgradeable returns a bool indicating if the tag represents an upgrade to the version
func upgradeable(versionNumerics []int, tag, suffix string, versionPrefix bool) (upgradeable bool) {
	var ignoreTags = map[string]struct{}{
		"latest": struct{}{},
	}
	_, ok := ignoreTags[tag]
	if !ok && prefixSuffixMatch(tag, suffix, versionPrefix) {
		upgradeable = numericalVersionUpgrade(versionNumerics, tag, suffix, versionPrefix)
	}
	return
}

//numericalVersionUpgrade returns a boolean after attempting to match 2 slices of version numerics together.
// If a match is going to be found, there must exist the same number of numeric values
//  e.g. 2.60.2 would match 2.60.3, but 2.60.2 wouldn't match 2.61
// There also has to be a numeric in the 'tag' slice (potential upgrade target) that is greater than the current
// version slice
//  e.g. 2.61.0 is an upgrade to 2.60.3 as the numeric in index=1 is greater. 2.60.2 wouldn't be an upgrade as none of
//  the numerical values (in any index) are greater than in its counterpart
func numericalVersionUpgrade(versionNumericValues []int, tag, suffix string, versionPrefix bool) (numericUpgrade bool) {
	tagNumerics := versionNumerics(tag, suffix, versionPrefix)
	if len(versionNumericValues) == len(tagNumerics) {
		for i, versionNumeric := range versionNumericValues {
			if tagNumerics[i] == versionNumeric {
				continue
			} else if tagNumerics[i] > versionNumeric {
				numericUpgrade = true
			}
			break
		}
	}
	return
}

//prefixSuffixMatch returns a bool indicating whether the tag string exhibits the same prefix/suffix properties as
// those specified
func prefixSuffixMatch(tag, suffixx string, versionPrefixx bool) (prefixSuffixMatch bool) {
	var prefixMatch bool
	tagPrefix := versionPrefix(tag)
	if versionPrefixx {
		prefixMatch = tagPrefix
	} else {
		prefixMatch = !tagPrefix
	}
	prefixSuffixMatch = prefixMatch && suffixx == suffix(tag)
	return
}

//imageToResultsMap returns a map of image <--> InspectrResult type, constructed from what's deemed to be valid pods
// in rs json from k8s master
func imageToResultsMap(jsonData *Data) (imageToResultsMap map[string][]InspectrResult) {
	var ignoreNamespaces = map[string]struct{}{
		"kube-system": struct{}{},
	}
	var allowedPodPhases = map[string]struct{}{
		"Running": struct{}{},
	}
	imageToResultsMap = make(map[string][]InspectrResult)
	projectName := projectName()
	clusterName := clusterName()
	for _, item := range jsonData.Items {
		metadata := item.Metadata
		namespace := metadata.Namespace
		_, ok := ignoreNamespaces[namespace]
		if !ok {
			phase := item.Status.Phase
			_, ok := allowedPodPhases[phase]
			if ok {
				for _, container := range item.Spec.Containers {
					containerImage := container.Image
					splitImage := strings.Split(containerImage, ":")
					if len(splitImage) > 1 {
						image := imageFromURI(containerImage)
						inspectrResult := InspectrResult{image, namespace,
							1, nil, versionFromURI(splitImage)}
						clusterImageString := projectName + ":" + clusterName + ":" + image + ":" +
							podName(metadata.Name) + ":" + container.Name
						inspectrResults, ok := imageToResultsMap[clusterImageString]
						if !ok {
							inspectrResults = make([]InspectrResult, 0)
						}
						inspectrResults = addInspectrResult(inspectrResults, inspectrResult)
						imageToResultsMap[clusterImageString] = inspectrResults
					}
				}
			}
		}
	}
	return
}

//podName returns a generic pod name from an actual full pod name
//
// This is assuming that all pods are suffixed by 2 strings separated by "-"
// e.g. [podname]-3229788801-zl7bq
func podName(fullPodName string) (podName string) {
	podNameSplitStrings := strings.Split(fullPodName, "-")
	var buffer bytes.Buffer
	first := true
	for index, podNameSplitString := range podNameSplitStrings {
		if index < len(podNameSplitStrings)-2 {
			if !first {
				buffer.WriteString("-")
			}
			first = false
			buffer.WriteString(podNameSplitString)
		} else {
			break
		}
	}
	podName = buffer.String()
	return
}

//clusterName returns the name of the cluster the inspectr application is running in
func clusterName() (clusterName string) {
	clusterName = computeMetadata("instance/attributes/cluster-name")
	return
}

//projectName returns the name of the project the inspectr application is running in
func projectName() (projectName string) {
	projectName = computeMetadata("project/project-id")
	return
}

//computeMetadata fires off an http request to the compute metadata 'api' and
// returns the string response, or "UNK" if an error is encountered
func computeMetadata(urlSuffix string) (computeMetadata string) {
	computeMetadata = "UNK"
	req, _ := http.NewRequest("GET", "http://metadata/computeMetadata/v1/"+urlSuffix,
		nil)
	req.Header.Set("Metadata-Flavor", "Google")
	client := &http.Client{}
	var resp *http.Response
	var err error
	resp, err = client.Do(req)
	if err == nil {
		defer resp.Body.Close()
		metaBytes, err := ioutil.ReadAll(resp.Body)
		if err == nil {
			computeMetadata = string(metaBytes)
		}
	}
	return
}

//addInspectrResult returns a slice of InspectrResult types, after either augmenting an existing item, or creating
// a new one
func addInspectrResult(inspectrResults []InspectrResult, inspectrResult InspectrResult) []InspectrResult {
	augmented := false
	for i, result := range inspectrResults {
		if result.Namespace == inspectrResult.Namespace && result.Version == inspectrResult.Version {
			inspectrResults[i].Quantity++
			augmented = true
			break
		}
	}
	if !augmented {
		inspectrResults = append(inspectrResults, inspectrResult)
	}
	return inspectrResults
}

//bodyFromMaster returns a ReadCloser from the k8s master's rs, and an error
func bodyFromMaster() (r io.ReadCloser, err error) {
	var caCert []byte
	caCert, err = ioutil.ReadFile("/var/run/secrets/kubernetes.io/serviceaccount/ca.crt")
	if err == nil {
		caCertPool := x509.NewCertPool()
		caCertPool.AppendCertsFromPEM(caCert)
		client := &http.Client{
			Transport: &http.Transport{
				TLSClientConfig: &tls.Config{
					RootCAs: caCertPool,
				},
			},
			Timeout: 30 * time.Second,
		}
		req, _ := http.NewRequest("GET", "https://kubernetes.default/api/v1/pods", nil)
		var token []byte
		token, err = ioutil.ReadFile("/var/run/secrets/kubernetes.io/serviceaccount/token")
		if err == nil {
			req.Header.Set("Authorization", "Bearer "+string(token))
			var resp *http.Response
			resp, err = client.Do(req)
			if err == nil {
				r = resp.Body
			}
		}
	}
	return
}

//decodeData returns a Data type, decoded from the specified Reader, and an error
func decodeData(r io.Reader) (x *Data, err error) {
	x = new(Data)
	err = json.NewDecoder(r).Decode(x)
	return
}

//decodeDockerTag returns a DockerTag type, decoded from the specified Reader, and an error
func decodeDockerTag(r io.Reader) ([]DockerTag, error) {
	x := new([]DockerTag)
	err := json.NewDecoder(r).Decode(x)
	return *x, err
}

//decodeQuayTag returns a QuayTag type, decoded from the specified Reader, and an error
func decodeQuayTag(r io.Reader) (quayTags []QuayTag, err error) {
	x := new(List)
	err = json.NewDecoder(r).Decode(x)
	for _, tag := range []string(x.Tags) {
		var quayTag = new(QuayTag)
		quayTag.Name = tag
		quayTags = append(quayTags, *quayTag)
	}
	return
}

//decodeGcrTag returns a GcrTag type, decoded from the specified Reader, and an error
func decodeGcrTag(r io.Reader) (gcrTags []GcrTag, err error) {
	x := new(Gcr)
	err = json.NewDecoder(r).Decode(x)
	for _, tag := range []string(x.Tags) {
		var gcrTag = new(GcrTag)
		gcrTag.Name = tag
		gcrTags = append(gcrTags, *gcrTag)
	}
	return
}

//dockerTagSlice returns an AvailableImageData slice representing all available tags for the specified repo
func dockerTagSlice(repo string) (imagesData []AvailableImageData, err error) {
	imageURI := "https://registry.hub.docker.com/v1/repositories/" + repo + "/tags"
	resp, err := http.Get(imageURI)
	if err == nil {
		if resp.StatusCode == 200 {
			defer resp.Body.Close()
			var dockerTags []DockerTag
			dockerTags, err = decodeDockerTag(resp.Body)
			if err == nil {
				for _, dockerTag := range []DockerTag(dockerTags) {
					imagesData = append(imagesData, dockerTag)
				}
			}
		} else {
			glog.Warning("bad status code (" + strconv.Itoa(resp.StatusCode) + ") trying to access " + imageURI)
		}
	}
	return
}

//quayTagSlice returns an AvailableImageData slice representing all available tags for the specified repo
func quayTagSlice(repo string) (imagesData []AvailableImageData, err error) {
	repo = strings.Replace(repo, "quay.io/", "", 1)
	imageURI := "https://quay.io/v2/" + repo + "/tags/list"
	resp, err := http.Get(imageURI)
	if err == nil {
		if resp.StatusCode == 200 {
			defer resp.Body.Close()
			var quayTags []QuayTag
			quayTags, err = decodeQuayTag(resp.Body)
			if err == nil {
				for _, quayTag := range []QuayTag(quayTags) {
					imagesData = append(imagesData, quayTag)
				}
			}
		} else {
			glog.Warning("bad status code (" + strconv.Itoa(resp.StatusCode) + ") trying to access " + imageURI)
		}
	}
	return
}

//gcrTagSlice returns an AvailableImageData slice representing all available tags for the specified repo
func gcrTagSlice(repo string) (imagesData []AvailableImageData, err error) {
	repo = strings.Replace(repo, "gcr.io/", "", 1)
	imageURI := "https://gcr.io/v2/" + repo + "/tags/list"
	resp, err := http.Get(imageURI)
	if err == nil {
		if resp.StatusCode == 200 {
			defer resp.Body.Close()
			var gcrTags []GcrTag
			gcrTags, err = decodeGcrTag(resp.Body)
			if err == nil {
				for _, gcrTag := range []GcrTag(gcrTags) {
					imagesData = append(imagesData, gcrTag)
				}
			}
		} else {
			glog.Warning("bad status code (" + strconv.Itoa(resp.StatusCode) + ") trying to access " + imageURI)
		}
	}
	return
}

//outputResults outputs the specified results to various places, provided there's results and/or current timestamp is
//within the scheduled alert window
// It doesn't return anything.
func outputResults(upgradeMap map[string][]InspectrResult, webhookID string, withinAlertWindow bool) {
	if len(upgradeMap) > 0 || withinAlertWindow {
		glog.Info("latest results: " + fmt.Sprintf("%#v", upgradeMap))
		postResultToSlack(upgradeMap, webhookID)
	}
}

//postResultToSlack posts a string representation of the inspectrResultMap to slack
//It doesn't return anything.
func postResultToSlack(upgradeMap map[string][]InspectrResult, webhookID string) {
	postStringToSlack(fmt.Sprintf("%#v", upgradeMap), webhookID)
}

//postStringToSlack posts the specified string to the specified slack webhook.
// //It doesn't return anything.
func postStringToSlack(payload, webhookID string) {
	if len(webhookID) > 0 {
		slackMsg := SlackMsg{payload, "inspectr"}
		bytesBuff := new(bytes.Buffer)
		json.NewEncoder(bytesBuff).Encode(slackMsg)
		_, err := http.Post("https://hooks.slack.com/services/"+webhookID,
			"application/json; charset=utf-8", bytesBuff)
		if err != nil {
			glog.Error(err)
		}
	} else {
		glog.Info("not outputting to slack as the webhookID I've got is empty. Have you set the " +
			"INSPECTR_SLACK_WEBHOOK_ID env var?")
	}

}

//imageFromURI returns the image 'name' from a URI. E.g. 'eversc/inspectr' from the URI: 'eversc/inspectr:v0.0.1-alpha'
func imageFromURI(imageURI string) (image string) {
	image = strings.Split(imageURI, ":")[0]
	return
}

//versionFromURI returns the image tag from a URI. E.g. 'v0.0.1-alpha' from the URI: 'eversc/inspectr:v0.0.1-alpha'
func versionFromURI(splitImage []string) (version string) {
	version = splitImage[1]
	return
}

//summaryFromInspectrMapKey returns a 'summary' string for use on an issue in a bugtracking service, e.g. JIRA
func summaryFromInspectrMapKey(key string) (summary string) {
	var buffer bytes.Buffer
	buffer.WriteString("inspectr upgrade")
	buffer.WriteString(" \\\\[image\\\\]: ")
	buffer.WriteString(imageFromInspectrMapKey(key))
	buffer.WriteString(" \\\\[project\\\\]: ")
	buffer.WriteString(projectFromInspectrMapKey(key))
	buffer.WriteString(" \\\\[cluster\\\\]: ")
	buffer.WriteString(clusterFromInspectrMapKey(key))
	buffer.WriteString(" \\\\[pod\\\\]: ")
	buffer.WriteString(podFromInspectrMapKey(key))
	buffer.WriteString(" \\\\[container\\\\]: ")
	buffer.WriteString(containerFromInspectrMapKey(key))

	summary = buffer.String()
	return
}

func projectFromInspectrMapKey(mapKey string) (project string) {
	project = strings.Split(mapKey, ":")[0]
	return
}

//clusterFromInspectrMapKey returns the cluster string from an inspectr map key
func clusterFromInspectrMapKey(mapKey string) (cluster string) {
	cluster = strings.Split(mapKey, ":")[1]
	return
}

//imageFromInspectrMapKey returns the image string from an inspectr map key
func imageFromInspectrMapKey(mapKey string) (image string) {
	image = strings.Split(mapKey, ":")[2]
	return
}

//podFromInspectrMapKey returns the pod string from an inspectr map key
func podFromInspectrMapKey(mapKey string) (pod string) {
	pod = strings.Split(mapKey, ":")[3]
	return
}

//containerFromInspectrMapKey returns the container string from an inspectr map key
func containerFromInspectrMapKey(mapKey string) (container string) {
	container = strings.Split(mapKey, ":")[4]
	return
}

//reportResults updates or creates a JIRA issue based on the upgradeMap provided
//
//jiraParamString should be of the form:
//     user|pass|project|issueType|otherFieldKey:otherFieldValue,otherFieldKey:otherFieldValue
//
//  user, pass, project and issueType are mandatory
//  1:n otherField k:v are optional  (but could be mandatory in your JIRA project)
//
//  otherField keys should be as they appear in the JIRA UI
func reportResults(upgradeMap map[string][]InspectrResult, jiraURL, jiraParamString, webhookID string) {
	jiraParamStrings := strings.Split(jiraParamString, "|")
	var resp *jira.Response
	var err error
	if len(jiraParamStrings) > 3 {
		var jiraClient *jira.Client
		jiraClient, err = jira.NewClient(nil, jiraURL)
		if err == nil {
			jiraClient.Authentication.SetBasicAuth(jiraParamStrings[0], jiraParamStrings[1])
			project := jiraParamStrings[2]
			issueType := jiraParamStrings[3]
			var otherFields string
			if len(jiraParamStrings) > 4 {
				otherFields = jiraParamStrings[4]
			}
			for k, v := range upgradeMap {
				summary := summaryFromInspectrMapKey(k)
				var issues []jira.Issue
				issues, resp, err = jiraClient.Issue.Search("summary ~ \""+summary+"\" AND project = "+project, nil)
				if err == nil {
					if len(issues) == 1 {
						for _, inspectrResult := range v {
							var resultMentioned bool
							issue := issues[0]
							if issue.Fields.Comments == nil {
								resultMentioned = stringContainsInspectrResult(issue.Fields.Description, inspectrResult)
							} else {
								for _, comment := range issue.Fields.Comments.Comments {
									if stringContainsInspectrResult(comment.Body, inspectrResult) {
										resultMentioned = true
										break
									}
								}
							}
							if !resultMentioned {
								addInspectrCommentToIssue(issue.Key, inspectrResult, jiraClient, jiraURL, webhookID)
							}
						}
					} else if len(issues) == 0 {
						createIssue(project, summary, issueType, otherFields, k, v, jiraClient, jiraURL, webhookID)
					} else {
						//TODO: log, there shouldn't be multiple result
					}
				}
			}
		} else if len(jiraParamStrings) > 0 {
			err = errors.New("JIRA param strings specified but not enough params found. " +
				"Usage: user|pass|project|issueType|otherFieldKey:otherFieldValue,otherFieldKey:otherFieldValue...")
		}
	}
	logIfFail(resp, err)
}

//stringContainsInspectrResult returns a bool indicating whether the inspectrResult specified
// is 'represented' in the string, based on a certain set of rules
func stringContainsInspectrResult(commentOrDescString string, inspectrResult InspectrResult) (resultMentioned bool) {
	resultMentioned = strings.Contains(commentOrDescString, "Namespace: "+inspectrResult.Namespace) &&
		strings.Contains(commentOrDescString, "Name: "+inspectrResult.Name) &&
		strings.Contains(commentOrDescString, upgradesString(inspectrResult))
	return
}

//addInspectrCommentToIssue adds a comment to the JIRA specified, based on the InspectrResult.
// It doesn't return anything
func addInspectrCommentToIssue(issueKey string, inspectrResult InspectrResult, jiraClient *jira.Client, jiraURL, webhookID string) {
	_, resp, err := jiraClient.Issue.AddComment(issueKey, commentFromInspectrResult(inspectrResult))
	if err == nil {
		postStringToSlack("just commented on "+jiraURL+"browse/"+issueKey, webhookID)
	}
	logIfFail(resp, err)
}

//upgradesString returns a comma sep string of the upgrade versions, prefixed by "Upgrades: "
func upgradesString(inspectrResult InspectrResult) (upgradesString string) {
	var buffer bytes.Buffer
	buffer.WriteString("Upgrades: ")
	first := true
	for _, upgrade := range inspectrResult.Upgrades {
		if !first {
			buffer.WriteString(", ")
		}
		first = false
		buffer.WriteString(upgrade)
	}
	upgradesString = buffer.String()
	return
}

//commentFromInspectrResult returns a JIRA comment containing the InspectrResult details
func commentFromInspectrResult(inspectrResult InspectrResult) (comment *jira.Comment) {
	newLineString := "\n"
	var buffer bytes.Buffer
	buffer.WriteString("{code}")
	buffer.WriteString("Name: ")
	buffer.WriteString(inspectrResult.Name)
	buffer.WriteString(newLineString)
	buffer.WriteString("Namespace: ")
	buffer.WriteString(inspectrResult.Namespace)
	buffer.WriteString(newLineString)
	buffer.WriteString("Quantity: ")
	buffer.WriteString(strconv.FormatInt(inspectrResult.Quantity, 10))
	buffer.WriteString(newLineString)
	buffer.WriteString(upgradesString(inspectrResult))
	buffer.WriteString(newLineString)
	buffer.WriteString("Version: ")
	buffer.WriteString(inspectrResult.Version)
	buffer.WriteString(newLineString)
	buffer.WriteString("{code}")
	comment = new(jira.Comment)
	comment.Body = buffer.String()
	return
}

//infraDetailsString returns a string with k8s 'infrastructure' summmary details
func infraDetailsString(mapkey string) (infraDetailsString string) {
	newLineString := "\n"
	var buffer bytes.Buffer
	buffer.WriteString("project: ")
	buffer.WriteString(projectFromInspectrMapKey(mapkey))
	buffer.WriteString(newLineString)
	buffer.WriteString("image: ")
	buffer.WriteString(imageFromInspectrMapKey(mapkey))
	buffer.WriteString(newLineString)
	buffer.WriteString("cluster: ")
	buffer.WriteString(clusterFromInspectrMapKey(mapkey))
	buffer.WriteString(newLineString)
	buffer.WriteString("pod: ")
	buffer.WriteString(podFromInspectrMapKey(mapkey))
	buffer.WriteString(newLineString)
	buffer.WriteString("container: ")
	buffer.WriteString(containerFromInspectrMapKey(mapkey))
	buffer.WriteString(newLineString)
	buffer.WriteString(newLineString)
	infraDetailsString = buffer.String()
	return
}

//createIssue creates a new JIRA issue with the necessary fields/summary/desc.
// It doesn't return anything.
func createIssue(project, summary, issueType, otherFields, mapKey string,
	inspectrResults []InspectrResult, jiraClient *jira.Client,
	jiraURL, webhookID string) {
	var err error
	var resp *jira.Response
	var meta *jira.CreateMetaInfo
	meta, resp, err = jiraClient.Issue.GetCreateMeta(project)
	if err == nil {
		metaProject := meta.GetProjectWithKey(project)
		metaIssuetype := metaProject.GetIssueTypeWithName(issueType)
		fieldsConfig := make(map[string]string, 0)
		var buffer bytes.Buffer
		buffer.WriteString(infraDetailsString(mapKey))
		for _, inspectrResult := range inspectrResults {
			buffer.WriteString(commentFromInspectrResult(inspectrResult).Body)
		}
		fieldsConfig["Summary"] = summary
		fieldsConfig["Issue Type"] = issueType
		fieldsConfig["Description"] = buffer.String()
		fieldsConfig["Project"] = project
		if otherFields != "" {
			for _, otherField := range strings.Split(otherFields, ",") {
				keyValueStrings := strings.Split(otherField, ":")
				if len(keyValueStrings) == 2 {
					fieldsConfig[keyValueStrings[0]] = keyValueStrings[1]
				}
			}
		}
		var issue *jira.Issue
		issue, err = jira.InitIssueWithMetaAndFields(metaProject, metaIssuetype, fieldsConfig)
		if err == nil {
			issue, resp, err = jiraClient.Issue.Create(issue)
			if err == nil {
				postStringToSlack("just created "+jiraURL+"browse/"+issue.Key, webhookID)
			}
		}
	}
	logIfFail(resp, err)
}

//logIfFail outputs to glog if err is not nil, also adding a response string if that's not nil
func logIfFail(resp *jira.Response, err error) {
	if err != nil {
		bodyString := "UNK"
		if resp != nil {
			body, _ := ioutil.ReadAll(resp.Body)
			bodyString = string(body)
		}
		glog.Errorf("response: %q, error: %q", bodyString, err)
	}
}
