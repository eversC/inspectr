package main

import "testing"

func TestDockerTag(t *testing.T) {
	var dockerTag DockerTag
	expected := "docker"
	dockerTag.Name = expected
	tag := dockerTag.tag()
	if tag != expected {
		t.Errorf("Docker tag was incorrect, got: %s, want: %s.", tag, expected)
	}
}

func TestGcrTag(t *testing.T) {
	var gcrTag GcrTag
	expected := "gcr"
	gcrTag.Name = expected
	tag := gcrTag.tag()
	if tag != expected {
		t.Errorf("Gcr tag was incorrect, got: %s, want: %s.", tag, expected)
	}
}

func TestQuayTag(t *testing.T) {
	var quayTag QuayTag
	expected := "quay"
	quayTag.Name = expected
	tag := quayTag.tag()
	if tag != expected {
		t.Errorf("Quay tag was incorrect, got: %s, want: %s.", tag, expected)
	}
}

var sleepingTimes = []struct {
	withinWindow bool
	expected     int
}{
	{true, 360},
	{false, 60},
}

func TestSleepTime(t *testing.T) {
	for _, sleepingTime := range sleepingTimes {
		if v := sleepTime(sleepingTime.withinWindow); v != sleepingTime.expected {
			t.Errorf("sleepTime(%t) returned %d, expected %d",
				sleepingTime.withinWindow, v, sleepingTime.expected)
		}
	}
}

var podnames = []struct {
	fullpodname string
	podname     string
}{
	{"banana-3229788801-zl7bq", "banana"},
	{"apples-and-pears-3229788801-zl7bq", "apples-and-pears"},
}

func TestPodName(t *testing.T) {
	for _, podname := range podnames {
		if v := podName(podname.fullpodname); v != podname.podname {
			t.Errorf("podName(%s) returned %s, expected %s", podname.fullpodname, v,
				podname.podname)
		}
	}
}

var cappedslackstrings = []struct {
	candidates   []string
	cappedstring string
}{
	{[]string{"v0.0.1"}, "v0.0.1"},
}

func TestCappedSlackString(t *testing.T) {
	for _, cappedslackstring := range cappedslackstrings {
		if v := cappedSlackString(cappedslackstring.candidates); v !=
			cappedslackstring.cappedstring {
			t.Errorf("cappedSlackString(%v) returned %s, expected %s",
				cappedslackstring.candidates, v, cappedslackstring.cappedstring)
		}
	}
}
