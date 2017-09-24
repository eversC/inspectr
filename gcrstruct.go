package main

type Gcr struct {
	Child    []interface{} `json:"child"`
	Manifest struct {
		ImageMap map[string]Image
	} `json:"manifest"`
	Name string   `json:"name"`
	Tags []string `json:"tags"`
}

type Image struct {
	ImageSizeBytes int64    `json:"imageSizeBytes,string"`
	LayerID        string   `json:"layerId"`
	MediaType      string   `json:"mediaType"`
	Tag            []string `json:"tag"`
	TimeCreatedMs  int64    `json:"timeCreatedMs,string"`
	TimeUploadedMs int64    `json:"timeUploadedMs,string"`
}