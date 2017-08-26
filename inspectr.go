package main

import (
	"bytes"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
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

//Pod type
type Pod struct {
	ImageURI string
	Name string
	Namespace string
	Phase string
}

//Data type representing the json schema of https://[master]/api/v1/pods
type Data struct {
	APIVersion string `json:"apiVersion"`
	Items      []struct {
		Metadata struct {
			Annotations struct {
				KubernetesIoCreatedBy string `json:"kubernetes.io/created-by"`
			} `json:"annotations"`
			CreationTimestamp time.Time `json:"creationTimestamp"`
			GenerateName      string    `json:"generateName"`
			Labels            struct {
				App             string `json:"app"`
				PodTemplateHash int64  `json:"pod-template-hash,string"`
			} `json:"labels"`
			Name            string `json:"name"`
			Namespace       string `json:"namespace"`
			OwnerReferences []struct {
				APIVersion         string `json:"apiVersion"`
				BlockOwnerDeletion bool   `json:"blockOwnerDeletion"`
				Controller         bool   `json:"controller"`
				Kind               string `json:"kind"`
				Name               string `json:"name"`
				UID                string `json:"uid"`
			} `json:"ownerReferences"`
			ResourceVersion int64  `json:"resourceVersion,string"`
			SelfLink        string `json:"selfLink"`
			UID             string `json:"uid"`
		} `json:"metadata"`
		Spec struct {
			Containers []struct {
				Image           string `json:"image"`
				ImagePullPolicy string `json:"imagePullPolicy"`
				Name            string `json:"name"`
				Ports           []struct {
					ContainerPort int64  `json:"containerPort"`
					Protocol      string `json:"protocol"`
				} `json:"ports"`
				ReadinessProbe struct {
					FailureThreshold int64 `json:"failureThreshold"`
					HTTPGet          struct {
						Path   string `json:"path"`
						Port   int64  `json:"port"`
						Scheme string `json:"scheme"`
					} `json:"httpGet"`
					PeriodSeconds    int64 `json:"periodSeconds"`
					SuccessThreshold int64 `json:"successThreshold"`
					TimeoutSeconds   int64 `json:"timeoutSeconds"`
				} `json:"readinessProbe"`
				Resources struct {
					Limits struct {
						CPU    string  `json:"string"`
						Memory string `json:"memory"`
					} `json:"limits"`
					Requests struct {
						CPU    string `json:"cpu"`
						Memory string `json:"memory"`
					} `json:"requests"`
				} `json:"resources"`
				TerminationMessagePath   string `json:"terminationMessagePath"`
				TerminationMessagePolicy string `json:"terminationMessagePolicy"`
				VolumeMounts             []struct {
					MountPath string `json:"mountPath"`
					Name      string `json:"name"`
				} `json:"volumeMounts"`
			} `json:"containers"`
			DNSPolicy       string `json:"dnsPolicy"`
			NodeName        string `json:"nodeName"`
			RestartPolicy   string `json:"restartPolicy"`
			SchedulerName   string `json:"schedulerName"`
			SecurityContext struct {
			} `json:"securityContext"`
			ServiceAccount                string `json:"serviceAccount"`
			ServiceAccountName            string `json:"serviceAccountName"`
			TerminationGracePeriodSeconds int64  `json:"terminationGracePeriodSeconds"`
			Tolerations                   []struct {
				Effect            string `json:"effect"`
				Key               string `json:"key"`
				Operator          string `json:"operator"`
				TolerationSeconds int64  `json:"tolerationSeconds"`
			} `json:"tolerations"`
			Volumes []struct {
				GcePersistentDisk struct {
					FsType    string `json:"fsType"`
					Partition int64  `json:"partition"`
					PdName    string `json:"pdName"`
				} `json:"gcePersistentDisk"`
				Name string `json:"name"`
			} `json:"volumes"`
		} `json:"spec"`
		Status struct {
			Conditions []struct {
				LastProbeTime      interface{} `json:"lastProbeTime"`
				LastTransitionTime time.Time   `json:"lastTransitionTime"`
				Status             string        `json:"string"`
				Type               string      `json:"type"`
			} `json:"conditions"`
			ContainerStatuses []struct {
				ContainerID string `json:"containerID"`
				Image       string `json:"image"`
				ImageID     string `json:"imageID"`
				LastState   struct {
				} `json:"lastState"`
				Name         string `json:"name"`
				Ready        bool   `json:"ready"`
				RestartCount int64  `json:"restartCount"`
				State        struct {
					Running struct {
						StartedAt time.Time `json:"startedAt"`
					} `json:"running"`
				} `json:"state"`
			} `json:"containerStatuses"`
			HostIP    net.IP    `json:"hostIP"`
			Phase     string    `json:"phase"`
			PodIP     net.IP    `json:"podIP"`
			QosClass  string    `json:"qosClass"`
			StartTime time.Time `json:"startTime"`
		} `json:"status"`
	} `json:"items"`
	Kind     string `json:"kind"`
	Metadata struct {
		ResourceVersion int64  `json:"resourceVersion,string"`
		SelfLink        string `json:"selfLink"`
	} `json:"metadata"`
}

func main(){
	fmt.Println("hello inspectr")

	bodyReader, err := bodyFromMaster()
	if err != nil {
		panic(err)
	}
	defer bodyReader.Close()
	jsonData, err := decodeData(bodyReader)
	if err != nil {
		panic(err)
	}
	imageToPodsMap := imageToPodsMap(jsonData)
	postToSlack(fmt.Sprintf("%#v", imageToPodsMap), "[webhookId]")
	//TODO: create string/slice map that'll contain the image string and available upgrades slice
	for k, v := range imageToPodsMap{
		//availImages, err := dockerTagSlice(strings.Split(k, ":")[0])
		availImages, err := dockerTagSlice("eversc/inspectr")
		fmt.Println(v)
		if err != nil{
			panic(err)
		}
		//TODO: add this to upgradeable string/slice map
		upgradeCandidateSlice(strings.Split(k, ":")[1], []AvailableImageData(availImages))
	}

	//TODO: using upgradeable string/slice map, post out to slack
	//postToSlack(fmt.Sprintf("%#v", dockerTagSlice), "[webhookId]")

}

//Dockertag implementation of AvailableImageData
func (dockerTag DockerTag) tag() string {
	return dockerTag.Name
}

//upgradeCandidateSlice returns a slice of AvailableImageData types that are deemed to be upgrades to the version
//specified
func upgradeCandidateSlice(version string, availImagesData []AvailableImageData) (upgradeCandidates []AvailableImageData){
	for _, availImageData := range  availImagesData{
		if upgradeable(version, availImageData.tag()){
			upgradeCandidates = append(upgradeCandidates, availImageData)
		}
	}
	return
}

//upgradeable returns a bool indicating if the tag represents an upgrade to the version
func upgradeable(version, tag string) (upgradeable bool){
	upgradeable = false
	//TODO: rules for determining if the tag string indicates upgrade is possible
	return
}

//podSlice returns a slice of Pod types, constructed from what's deemed to be valid pods in rs json from k8s master
func imageToPodsMap(jsonData *Data) (imageToPodsMap map[string][]Pod){
	var ignoreNamespaces = map[string]struct{}{
		"kube-system": struct{}{},
	}
	var allowedPodPhases = map[string]struct{}{
		"Running": struct{}{},
	}
	imageToPodsMap = make(map[string][]Pod)
	for _, item := range jsonData.Items{
		metadata := item.Metadata
		namespace := metadata.Namespace
		_, ok := ignoreNamespaces[namespace]
		if !ok {
			phase := item.Status.Phase
			_, ok := allowedPodPhases[phase]
			if ok {
				for _, container := range item.Spec.Containers {
					image := container.Image
					pod := Pod{image, metadata.Name, namespace, phase}
					pods, _ := imageToPodsMap[image]
					if !podSliceContains(pods, pod){
						pods = append(pods, pod)
						imageToPodsMap[image] = pods
					}
				}
			}
		}
	}
	return
}

//podSliceContains returns a bool indicating whether the specified Pod is in the specified Pod slice
func podSliceContains(pods []Pod, pod Pod) (podExists bool){
	podExists = false
	for _, slicePod := range pods{
		if slicePod.ImageURI == pod.ImageURI && slicePod.Namespace == pod.Namespace{
			podExists = true
			break
		}
	}
	return
}

//bodyFromMaster returns a ReadCloser from the k8s master's rs, and an error
func bodyFromMaster() (r io.ReadCloser, err error){
	tr := &http.Transport{
		TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
	}
	client := &http.Client{Transport: tr}
	req, _ := http.NewRequest("GET", "https://[master ip]/api/v1/pods", nil)
	req.Header.Set("Authorization", "Bearer [token]")
	resp, err := client.Do(req)
	if err != nil {
		panic(err)
	}
	r = resp.Body
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
	resp, err := http.Get("https://registry.hub.docker.com/v1/repositories/" + repo + "/tags")
	if err != nil {
		panic(err)
	}
	defer resp.Body.Close()
	dockerTags, err := decodeDockerTag(resp.Body)

	for _, dockerTag := range []DockerTag(dockerTags){
		imagesData = append(imagesData, dockerTag)
		//imagesData[i] = dockerTag
	}
	return
}

//postToSlack posts the specified text string to the specified slack webhook.
//It doesn't return anything.
func postToSlack(text, webhookId string){
	bytesBuff := new(bytes.Buffer)
	slackMsg := SlackMsg{text, "inspectr"}
	json.NewEncoder(bytesBuff).Encode(slackMsg)
	_, err := http.Post("https://hooks.slack.com/services/" + webhookId,
		"application/json; charset=utf-8", bytesBuff)
	if err != nil {
		fmt.Print(err)
	}
}