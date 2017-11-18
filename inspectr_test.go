package main

import (
	"testing"

	jira "github.com/andygrunwald/go-jira"
)

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

func TestV2Tag(t *testing.T) {
	var v2Tag V2Tag
	expected := "v2"
	v2Tag.Name = expected
	tag := v2Tag.tag()
	if tag != expected {
		t.Errorf("Docker v2 tag was incorrect, got: %s, want: %s.", tag, expected)
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
	{[]string{"v0.0.1", "v0.0.2", "v0.0.3", "v0.0.4", "v0.0.5", "v0.0.6"},
		"v0.0.1, v0.0.2, v0.0.3, v0.0.4, v0.0.5 + 1 more"},
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

var imageURIs = []struct {
	imageURI string
	image    string
}{
	{"eversc/inspectr:v0.0.1-alpha", "eversc/inspectr"},
}

func TestImageFromURI(t *testing.T) {
	for _, imageURI := range imageURIs {
		if v := imageFromURI(imageURI.imageURI); v != imageURI.image {
			t.Errorf("imageFromURI(%s) returned %s, expected %s", imageURI.imageURI,
				v, imageURI.image)
		}
	}
}

var versions = []struct {
	splitImageStrings []string
	version           string
}{
	{[]string{"eversc/inspectr", "v0.0.1-alpha"}, "v0.0.1-alpha"},
}

func TestVersionFromURI(t *testing.T) {
	for _, version := range versions {
		if v := versionFromURI(version.splitImageStrings); v != version.version {
			t.Errorf("versionFromURI(%v) returned %s, expected %s",
				version.splitImageStrings, v, version.version)
		}
	}
}

var inspectrProjectMapKeys = []struct {
	inspectrMapKey string
	project        string
}{
	{"project:cluster:image:pod:container", "project"},
}

func TestProjectFromInspectrMapKey(t *testing.T) {
	for _, inspectrMapKey := range inspectrProjectMapKeys {
		if v := projectFromInspectrMapKey(inspectrMapKey.inspectrMapKey); v != inspectrMapKey.project {
			t.Errorf("projectFromInspectrMapKey(%s) returned %s, expected %s",
				inspectrMapKey.inspectrMapKey, v, inspectrMapKey.project)
		}
	}
}

var inspectrClusterMapKeys = []struct {
	inspectrMapKey string
	cluster        string
}{
	{"project:cluster:image:pod:container", "cluster"},
}

func TestClusterFromInspectrMapKey(t *testing.T) {
	for _, inspectrMapKey := range inspectrClusterMapKeys {
		if v := clusterFromInspectrMapKey(inspectrMapKey.inspectrMapKey); v != inspectrMapKey.cluster {
			t.Errorf("clusterFromInspectrMapKey(%s) returned %s, expected %s",
				inspectrMapKey.inspectrMapKey, v, inspectrMapKey.cluster)
		}
	}
}

var inspectrImageMapKeys = []struct {
	inspectrMapKey string
	image          string
}{
	{"project:cluster:image:pod:container", "image"},
}

func TestImageFromInspectrMapKey(t *testing.T) {
	for _, inspectrMapKey := range inspectrImageMapKeys {
		if v := imageFromInspectrMapKey(inspectrMapKey.inspectrMapKey); v != inspectrMapKey.image {
			t.Errorf("imageFromInspectrMapKey(%s) returned %s, expected %s",
				inspectrMapKey.inspectrMapKey, v, inspectrMapKey.image)
		}
	}
}

var inspectrPodMapKeys = []struct {
	inspectrMapKey string
	pod            string
}{
	{"project:cluster:image:pod:container", "pod"},
}

func TestPodFromInspectrMapKey(t *testing.T) {
	for _, inspectrMapKey := range inspectrPodMapKeys {
		if v := podFromInspectrMapKey(inspectrMapKey.inspectrMapKey); v != inspectrMapKey.pod {
			t.Errorf("podFromInspectrMapKey(%s) returned %s, expected %s",
				inspectrMapKey.inspectrMapKey, v, inspectrMapKey.pod)
		}
	}
}

var inspectrContainerMapKeys = []struct {
	inspectrMapKey string
	container      string
}{
	{"project:cluster:image:pod:container", "container"},
}

func TestContainerFromInspectrMapKey(t *testing.T) {
	for _, inspectrMapKey := range inspectrContainerMapKeys {
		if v := containerFromInspectrMapKey(inspectrMapKey.inspectrMapKey); v != inspectrMapKey.container {
			t.Errorf("containerFromInspectrMapKey(%s) returned %s, expected %s",
				inspectrMapKey.inspectrMapKey, v, inspectrMapKey.container)
		}
	}
}

var resultMentionedVars = []struct {
	commentBody             string
	inspectrResultName      string
	inspectrResultNamespace string
	upgrades                []string
	resultMentioned         bool
}{
	{"Name: banana\nNamespace: banana-namespace\nUpgrades: v0.0.1, v0.0.2",
		"banana", "banana-namespace", []string{"v0.0.1", "v0.0.2"}, true},
	{"Name: apples\nNamespace: apples-namespace\nUpgrades: v0.0.1, v0.0.2",
		"apples", "apples-namespace", []string{"v0.0.1", "v0.0.3"}, false},
	{"Name: pears\nUpgrades: v0.0.1, v0.0.2",
		"pears", "pears-namespace", []string{"v0.0.1", "v0.0.2"}, false},
	{"Namespace: banana-namespace\nUpgrades: v0.0.1, v0.0.2",
		"banana", "banana-namespace", []string{"v0.0.1", "v0.0.2"}, false},
	{"Name: banana\nNamespace: banana-namespace\nUpgrades: v0.0.1, v0.0.2",
		"apples", "pears-namespace", []string{"v0.0.1", "v0.0.2"}, false},
}

func TestResultMentioned(t *testing.T) {
	for _, resultMentionedVar := range resultMentionedVars {
		var issue = new(jira.Issue)
		var fields = new(jira.IssueFields)
		var comments = new(jira.Comments)
		var commentSlice = make([]*jira.Comment, 1)
		var comment = new(jira.Comment)
		comment.Body = resultMentionedVar.commentBody
		commentSlice[0] = comment
		comments.Comments = commentSlice
		fields.Comments = comments
		issue.Fields = fields
		var inspectrResult InspectrResult
		inspectrResult.Name = resultMentionedVar.inspectrResultName
		inspectrResult.Namespace = resultMentionedVar.inspectrResultNamespace
		inspectrResult.Upgrades = resultMentionedVar.upgrades

		if v := resultMentioned(issue, inspectrResult); v !=
			resultMentionedVar.resultMentioned {
			t.Errorf("resultMentioned(%+v\n, %+v)\n returned %t, expected %t",
				issue.Fields.Comments.Comments[0].Body, inspectrResult, v,
				resultMentionedVar.resultMentioned)
		}
	}
}
