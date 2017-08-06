package inspectr

import (
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"time"
)

type Pod struct {
	ImageUri string
	Name string
	Namespace string
	Phase string
}

type Data struct {
	ApiVersion string `json:"apiVersion"`
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
				ApiVersion         string `json:"apiVersion"`
				BlockOwnerDeletion bool   `json:"blockOwnerDeletion"`
				Controller         bool   `json:"controller"`
				Kind               string `json:"kind"`
				Name               string `json:"name"`
				Uid                string `json:"uid"`
			} `json:"ownerReferences"`
			ResourceVersion int64  `json:"resourceVersion,string"`
			SelfLink        string `json:"selfLink"`
			Uid             string `json:"uid"`
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
					HttpGet          struct {
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
						Cpu    string  `json:"string"`
						Memory string `json:"memory"`
					} `json:"limits"`
					Requests struct {
						Cpu    string `json:"cpu"`
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
			DnsPolicy       string `json:"dnsPolicy"`
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
	jsonData, err := decode(bodyReader)
	if err != nil {
		panic(err)
	}
	podSlice := podSlice(jsonData)
	fmt.Println(podSlice)
}

func podSlice(jsonData *Data) (pods []Pod){
	var ignoreNamespaces = map[string]struct{}{
		"kube-system": struct{}{},
	}
	var allowedPodPhases = map[string]struct{}{
		"Running": struct{}{},
	}

	for _, item := range jsonData.Items{
		metadata := item.Metadata
		namespace := metadata.Namespace
		_, ok := ignoreNamespaces[namespace]
		if !ok {
			phase := item.Status.Phase
			_, ok := allowedPodPhases[phase]
			if ok {
				for _, container := range item.Spec.Containers {
					pod := Pod{container.Image, metadata.Name, namespace, phase}
					pods = append(pods, pod)
				}
			}
		}
	}
	return
}

func bodyFromMaster() (r io.ReadCloser, err error){
	tr := &http.Transport{
		TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
	}
	client := &http.Client{Transport: tr}
	req, _ := http.NewRequest("GET", "https://[master ip]/api/v1/pods", nil)
	req.Header.Set("Authorization", "Bearer [token]")
	resp, err := client.Do(req)
	if err != nil {
		fmt.Println(err)
	}
	r = resp.Body
	return
}

func decode(r io.Reader) (x *Data, err error) {
	x = new(Data)
	err = json.NewDecoder(r).Decode(x)
	return
}