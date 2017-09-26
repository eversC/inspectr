package main

//Gcr type representing the json schema of http://gcr.io/v2/[image]/tags/list
type Gcr struct {
	Child    []interface{} `json:"child"`
	Manifest struct {
		ImageMap map[string]Image
	} `json:"manifest"`
	Name string   `json:"name"`
	Tags []string `json:"tags"`
}

//Image type representing the dynamic "sha256:[]":{} part of gcr.io tags list
type Image struct {
	ImageSizeBytes int64    `json:"imageSizeBytes,string"`
	LayerID        string   `json:"layerId"`
	MediaType      string   `json:"mediaType"`
	Tag            []string `json:"tag"`
	TimeCreatedMs  int64    `json:"timeCreatedMs,string"`
	TimeUploadedMs int64    `json:"timeUploadedMs,string"`
}
