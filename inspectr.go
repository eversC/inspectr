package main

import (
	"bytes"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"flag"
	"fmt"
	"github.com/golang/glog"
	"io"
	"io/ioutil"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"
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

//SlackMsg type
type SlackMsg struct {
	Text string `json:"text"`
	Username string `json:"username"`
}

//InspectrResult type
type InspectrResult struct{
	Name string
	Namespace string
	Quantity int64
	Upgrades []string
	Version string
}

func main(){
	flag.Parse()
	glog.Info("hello inspectr")
	registeredImages := make(map[string][]string)
	glog.Info("initialized local image registry cache")
	envKey := "INSPECTR_SLACK_WEBHOOK_ID"
	glog.Info("obtained slack webhook id")
	webhookID := os.Getenv(envKey)
	glog.Info("about to enter life-of-pod loop")
	for {
		time.Sleep(time.Duration(invokeInspectrProcess(&registeredImages, webhookID))*time.Second)
	}
}

//invokeInspectrProcess attempts to run through as much of the 'process' as it can. At appropriate points it may
// abort if it sees an error. If this happens, the error is logged, and a default/long time is returned as the sleep
// value.
// if everything goes okay, the sleep value returned is either pretty small (as inspectr should be quick to detect
// any 'unregistered' images, or a bit longer if current time is withinAlertWindow
func invokeInspectrProcess(registeredImages *map[string][]string, webhookID string)(sleep int){
	sleep = 300
	var k8sJSONData *Data
	var err error
	k8sJSONData, err = jsonData()
	if err == nil{
		withinAlertWindow := withinAlertWindow()
		var upgradeMap map[string][]InspectrResult
		upgradeMap, err = upgradesMap(imageToResultsMap(k8sJSONData))
		if err == nil{
			upgradeMap = filterUpgradesMap(upgradeMap, *registeredImages, withinAlertWindow)
			augmentInternalImageRegistry(upgradeMap, *registeredImages, withinAlertWindow)
			outputResults(upgradeMap, webhookID, withinAlertWindow)
			sleep = sleepTime(withinAlertWindow)
		}
	}
	if err != nil{
		glog.Error(err, ", going to sleep for " + strconv.Itoa(sleep) + " seconds")
	}
	return
}

//jsonData returns a Data struct based on what k8s master returns, and an error
func jsonData()(jsonData *Data, err error){
	bodyReader, err := bodyFromMaster()
	if err == nil{
		jsonData, err = decodeData(bodyReader)
		bodyReader.Close()
	}
	return jsonData, err
}

//sleepTime returns an int of the number of seconds to go to sleep for. Sleep is needed so the process isn't
// constantly running, and doesn't run more than once in the alert window
func sleepTime(withinAlertWindow bool)(sleepTime int){
	if withinAlertWindow{
		sleepTime = 360
	}else{
		sleepTime = 60
	}
	return
}

//withinAlertWindow returns a bool indicating whether current time is within the scheduled daily/weekly alert window
func withinAlertWindow()(withinAlertWindow bool){
	//TODO: get a weekday value (from env var?)
	weekday := -1
	now := time.Now()
	preferredAlertTime := time.Date(int(now.Year()), now.Month(), int(now.Day()),
		11, 30, 0, 0, now.Location())
	windowSize := 300
	windowEnd := preferredAlertTime.Add(time.Duration(windowSize)*time.Second)
	if weekday == -1 || int(now.Weekday()) == weekday{
		withinAlertWindow = now.After(preferredAlertTime) && now.Before(windowEnd)
	}
	return
}

//filterUpgradesMap returns a map which is based on the one specified, but has any already registered upgrade
// opportunities removed. If we're withinAlertWindow, no filtering happens
func filterUpgradesMap(upgradesMap map[string][]InspectrResult, registeredImages map[string][]string,
	withinAlertWindow bool)(filteredMap map[string][]InspectrResult){

	if withinAlertWindow{
		filteredMap = upgradesMap
	}else{
		filteredMap = make(map[string][]InspectrResult)
		for k, v := range upgradesMap{
			registeredResults, ok := registeredImages[k]
			if ok{
				filteredSlice := make([]InspectrResult, 0)
				for _, result := range v{
					if !sliceContainsResult(registeredResults, result){
						filteredSlice = append(filteredSlice, result)
					}
				}
				if len(filteredSlice) > 0{
					filteredMap[k] = filteredSlice
				}
			}else{
				filteredMap[k] = v
			}
		}
	}
	return
}

//sliceContainsResult returns a bool indicating whether the specified InspectrResult is 'registered' in the string slice
func sliceContainsResult(registeredResults []string, result InspectrResult)(containsResult bool){
	for _, registeredResult := range registeredResults{
		splitStrings := strings.Split(registeredResult, "|")
		if splitStrings[0] == result.Version && splitStrings[1] == result.Namespace{
			containsResult = true
			break
		}
	}
	return
}

//augmentInternalImageRegistry will, based on whether we're withinAlertWindow, replace the registeredImageMap or
// augment it with any InspectrResults that are missing, respectively
func augmentInternalImageRegistry(upgradesMap map[string][]InspectrResult, registeredImageMap map[string][]string,
	withinAlertWindow bool){

	if withinAlertWindow{
		registeredImageMap = registeredImages(upgradesMap)
	}else{
		for k, v := range upgradesMap{
			registeredResults, ok := registeredImageMap[k]
			if ok{
				for _, result := range v{
					if !sliceContainsResult(registeredResults, result){
						registeredResults = append(registeredResults, registeredImageString(result))
						registeredImageMap[k] = registeredResults
					}
				}
			}else{
				registeredImageMap[k] = stringSliceFromResultSlice(v)
			}
		}
	}
}

//registeredImages returns a string<-->string map that reflects the string<-->[]InspectrResult map
func registeredImages(upgradesMap map[string][]InspectrResult)(registeredImages map[string][]string){
	for k, v := range upgradesMap{
		registeredImageSlice := make([]string, 0)
		for _, upgradeResult := range v{
			registeredImageSlice = append(registeredImageSlice, registeredImageString(upgradeResult))
		}
		registeredImages[k] = registeredImageSlice
	}
	return
}

//stringSliceFromResultSlice returns a string slice that represents the specified []InspectrResult
func stringSliceFromResultSlice(resultSlice []InspectrResult)(registeredResults []string){
	for _, result := range resultSlice{
		registeredResults = append(registeredResults, registeredImageString(result))
	}
	return
}

//registeredImageString returns a string consisting of [result.Version]|[result.Namespace], helpful for the
// image registry cache
func registeredImageString(result InspectrResult)(resultString string){
	resultString = result.Version + "|" + result.Namespace
	return
}

//upgradesMap returns a string <--> []InspectrResult only for those images with upgrades available
func upgradesMap(imageToResultsMap map[string][]InspectrResult) (upgradesMap map[string][]InspectrResult, err error){
	upgradesMap = make(map[string][]InspectrResult)
	for k, v := range imageToResultsMap{
		imageString := strings.Split(k, ":")[1]
		availImages, err := dockerTagSlice(imageString)
		if err == nil{
			upgradesResults := make([]InspectrResult, 0)
			for _, result := range v{
				for _, upgradeVersion := range upgradeCandidateSlice(result.Version, []AvailableImageData(availImages)){
					result.Upgrades = append(result.Upgrades, upgradeVersion.tag())
				}
				if len(result.Upgrades) > 0{
					upgradesResults = append(upgradesResults, result)
				}
			}
			if len(upgradesResults) > 0{
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

//upgradeCandidateSlice returns a slice of AvailableImageData types that are deemed to be upgrades to the version
//specified
func upgradeCandidateSlice(version string, availImagesData []AvailableImageData) (upgradeCandidates []AvailableImageData){
	versionPrefix := versionPrefix(version)
	suffix := suffix(version)
	versionNumerics := versionNumerics(version, suffix, versionPrefix)
	for _, availImageData := range  availImagesData{
		if upgradeable(versionNumerics, availImageData.tag(), suffix, versionPrefix){
			upgradeCandidates = append(upgradeCandidates, availImageData)
		}
	}
	return
}

//versionPrefix returns a bool indicating whether the specified string is prefixed with "v"
func versionPrefix(version string)(versionPrefix bool){
	versionPrefix = strings.HasPrefix(version, "v")
	return
}

//suffix returns the suffix of the string, with the suffix being anything after (and including) a hyphen at the tail
// end of the string
func suffix(version string)(suffix string){
	suffixStrings := strings.Split(version, "-")
	if len(suffixStrings) > 1{
		suffix = suffixStrings[1]
	}
	return
}

//versionNumerics returns a slice of the numerical values in the specified version string
func versionNumerics(version, suffix string, versionPrefix bool)(versionNumerics []int){
	versionNumeric := version
	if versionPrefix{
		versionNumeric = versionNumeric[1:len(versionNumeric) - 1]
	}
	versionNumeric = strings.Trim(versionNumeric, suffix)
	versionStringSlice := strings.Split(versionNumeric, ".")
	for _,versionString := range versionStringSlice{
		if numeric, err := strconv.Atoi(versionString); err == nil {
			versionNumerics = append(versionNumerics, numeric)
		}
	}
	return
}

//upgradeable returns a bool indicating if the tag represents an upgrade to the version
func upgradeable(versionNumerics []int, tag, suffix string, versionPrefix bool) (upgradeable bool){
	var ignoreTags = map[string]struct{}{
		"latest": struct{}{},
	}
	_, ok := ignoreTags[tag]
	if !ok && prefixSuffixMatch(tag, suffix, versionPrefix){
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
func numericalVersionUpgrade(versionNumericValues []int, tag, suffix string, versionPrefix bool)(numericUpgrade bool){
	tagNumerics := versionNumerics(tag, suffix, versionPrefix)
	if len(versionNumericValues) == len(tagNumerics){
		for i, versionNumeric := range versionNumericValues{
			if tagNumerics[i] == versionNumeric{
				continue
			}else if tagNumerics[i] > versionNumeric {
				numericUpgrade = true
			}
			break
		}
	}
	return
}

//prefixSuffixMatch returns a bool indicating whether the tag string exhibits the same prefix/suffix properties as
// those specified
func prefixSuffixMatch(tag, suffixx string, versionPrefixx bool)(prefixSuffixMatch bool){
	prefixMatch := false
	tagPrefix := versionPrefix(tag)
	if versionPrefixx {
		prefixMatch = tagPrefix
	}else{
		prefixMatch = !tagPrefix
	}
	prefixSuffixMatch = prefixMatch && suffixx == suffix(tag)
	return
}

//imageToResultsMap returns a map of image <--> InspectrResult type, constructed from what's deemed to be valid pods
// in rs json from k8s master
func imageToResultsMap(jsonData *Data) (imageToResultsMap map[string][]InspectrResult){
	var ignoreNamespaces = map[string]struct{}{
		"kube-system": struct{}{},
	}
	var allowedPodPhases = map[string]struct{}{
		"Running": struct{}{},
	}
	imageToResultsMap = make(map[string][]InspectrResult)
	for _, item := range jsonData.Items{
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
						image := imageFromURI(container.Image)
						inspectrResult := InspectrResult{image, namespace,
							1, nil,versionFromURI(splitImage)}
						clusterImageString := clusterName() + ":" + image
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

//clusterName returns the name of the cluster the inspectr application is running in
func clusterName()(clusterName string){
	clusterName = "UNK"
	req, _ := http.NewRequest("GET", "http://metadata/computeMetadata/v1/instance/attributes/cluster-name",
		nil)
	req.Header.Set("Metadata-Flavor", "Google")
	client := &http.Client{}
	var resp *http.Response
	var err error
	resp, err = client.Do(req)
	defer resp.Body.Close()
	if err == nil{
		clusterBytes, err := ioutil.ReadAll(resp.Body)
		if err == nil{
			clusterName = string(clusterBytes)
		}
	}
	return
}

//addInspectrResult returns a slice of InspectrResult types, after either augmenting an existing item, or creating
// a new one
func addInspectrResult(inspectrResults []InspectrResult, inspectrResult InspectrResult) ([]InspectrResult){
	augmented := false
	for i, result := range inspectrResults{
		if result.Namespace == inspectrResult.Namespace && result.Version == inspectrResult.Version{
			inspectrResults[i].Quantity++
			augmented = true
			break
		}
	}
	if !augmented{
		inspectrResults = append(inspectrResults, inspectrResult)
	}
	return inspectrResults
}

//bodyFromMaster returns a ReadCloser from the k8s master's rs, and an error
func bodyFromMaster() (r io.ReadCloser, err error){
	err = nil
	var caCert []byte
	caCert, err = ioutil.ReadFile("/var/run/secrets/kubernetes.io/serviceaccount/ca.crt")
	if err == nil{
		caCertPool := x509.NewCertPool()
		caCertPool.AppendCertsFromPEM(caCert)
		client := &http.Client{
			Transport: &http.Transport{
				TLSClientConfig: &tls.Config{
					RootCAs:      caCertPool,
				},
			},
		}
		req, _ := http.NewRequest("GET", "https://kubernetes.default/api/v1/pods", nil)
		var token []byte
		token, err = ioutil.ReadFile("/var/run/secrets/kubernetes.io/serviceaccount/token")
		if err == nil {
			req.Header.Set("Authorization", "Bearer " + string(token))
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
func decodeDockerTag(r io.Reader) ([]DockerTag, error){
	x := new([]DockerTag)
	err := json.NewDecoder(r).Decode(x)
	return *x, err
}

//dockerTagSlice returns a DockerTag slice
func dockerTagSlice(repo string) (imagesData []AvailableImageData, err error){
	err = nil
	resp, err := http.Get("https://registry.hub.docker.com/v1/repositories/" + repo + "/tags")
	if err == nil {
		defer resp.Body.Close()
		var dockerTags []DockerTag
		dockerTags, err = decodeDockerTag(resp.Body)

		for _, dockerTag := range []DockerTag(dockerTags){
			imagesData = append(imagesData, dockerTag)
			//imagesData[i] = dockerTag
		}
	}
	return
}

//outputResults outputs the specified results to various places, provided there's results and/or current timestamp is
//within the scheduled alert window
// It doesn't return anything.
func outputResults(upgradeMap map[string][]InspectrResult, webhookID string, withinAlertWindow bool){
	if len(upgradeMap) > 0 || withinAlertWindow {
		glog.Info("latest results: " + fmt.Sprintf("%#v", upgradeMap))
		postToSlack(upgradeMap, webhookID)
	}
}

//postToSlack posts the specified text string to the specified slack webhook.
//It doesn't return anything.
func postToSlack(upgradeMap map[string][]InspectrResult, webhookID string){
	if len(webhookID) > 0{
		bytesBuff := new(bytes.Buffer)
		slackMsg := SlackMsg{fmt.Sprintf("%#v", upgradeMap), "inspectr"}
		json.NewEncoder(bytesBuff).Encode(slackMsg)
		_, err := http.Post("https://hooks.slack.com/services/" + webhookID,
			"application/json; charset=utf-8", bytesBuff)
		if err != nil {
			glog.Error(err)
		}
	}else{
		glog.Info("not outputting to slack as the webhookId I've got is empty. Have you set the " +
			"INSPECTR_SLACK_WEBHOOK_ID env var?")
	}
}

//imageFromURI returns the image 'name' from a URI. E.g. 'eversc/inspectr' from the URI: 'eversc/inspectr:v0.0.1-alpha'
func imageFromURI(imageURI string)(image string) {
	image = strings.Split(imageURI, ":")[0]
	return
}

//versionFromURI returns the image tag from a URI. E.g. 'v0.0.1-alpha' from the URI: 'eversc/inspectr:v0.0.1-alpha'
func versionFromURI(splitImage []string)(version string){
	version = splitImage[1]
	return
}