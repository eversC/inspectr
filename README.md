# inspectr

[![Join the chat at https://gitter.im/inspectr/Lobby](https://badges.gitter.im/inspectr/Lobby.svg)](https://gitter.im/inspectr/Lobby?utm_source=badge&utm_medium=badge&utm_campaign=pr-badge&utm_content=badge)

this is a binary that, when run in a k8s cluster, gets details of pods in the same cluster (via the k8s master/API), and alerts if certain conditions are met.

currently, these 'conditions' are only met if there's an upgrade to an image that a pod is running.

public image repo integrations: dockerhub, gcr, quay  (no private repo functionality yet)

hence, you MUST be running at least one public image from one of the aforementioned repo's, otherwise there's not much point in running inspectr

## environment variables

| name        |       default      | description  |
| ------------- |:-------------:| :-----:|
| INSPECTR_JIRA_PARAMS      |  | JIRA auth and other details required for posting to JIRA REST API. Default is for JIRA to not be enabled (also requires INSPECTR_JIRA_URL env var)|
| INSPECTR_JIRA_URL         |  | URL of your JIRA instance. Default is for JIRA to not be enabled (also requires INSPECTR_JIRA_PARAMS env var)|
| INSPECTR_SCHEDULE         | 1000 | If daily, format is hhmm. If weekly, format is pipe separated, e.g. tuesday|1430 |
| INSPECTR_SLACK_WEBHOOK_ID |  | id of the webhook you want alerts going to. Default is for slack outputs to be disabled |
| INSPECTR_TIMEZONE         | Local | from time package (zoneinfo.go): *"If the name is "" or "UTC", LoadLocation returns UTC. If the name is "Local", LoadLocation returns Local. Otherwise, the name is taken to be a location name corresponding to a file in the IANA Time Zone database, such as "America/New_York"*. |


## result grouping

results are unique by cluster/pod-name/container-name/namespace/image

results are stored (and appear in alerts) in a map where the key is cluster:image:pod-name:cluster-name


## running/alerting frequency

the binary outputs a full set of results daily or weekly, and new results
whenever they're discovered.

the frequency at which the full resultset is outputted can be configered by
using the environment variable:

* ___INSPECTR_SCHEDULE___

e.g.
"1430" = 14:30 daily

"tuesday|1430" = 14:30 every tuesday

weekday is not case-sensitive

defaults to "1000" (daily)


## alert cache

to prevent noise, the binary keeps a cache of the clusters/images that an alert has been produced for

if a cluster/image appears in the cache, it won't get alerted on again until the 'scheduled alert window'

when the binary first runs, the cache is empty, so essentially you'll get a full result alert every time a pod starts/restarts


## slack alerts

the binary needs to know the webhook id that you want the alerts going to

it looks for this id in the environment variables:

* ___INSPECTR_SLACK_WEBHOOK_ID___

this id is the string that comes after "https://hooks.slack.com/services/" in your webhook URL

if that's not set, the binary still runs, you just (obviously) won't see any alerts in your Slack channel. Inspectr results are still logged via glog.


## jira

the inspectr binary can create a JIRA detailing the image upgrades it finds, or update existing JIRAs that may have been created on previous runs (it will only update if there are any additional new versions found, though).

to enable this functionality, you'll need to set 2 environment variables:

* ___INSPECTR_JIRA_URL___
  * URL of your JIRA instance
* ___INSPECTR_JIRA_PARAMS___
  * Usage: user|pass|project|issueType|otherFieldKey:otherFieldValue,otherFieldKey:otherFieldValue...
  * mandatory: user, pass, project, issueType
  * oauth2 hasn't been integrated yet..
  * optional (as your JIRA instance may require them): otherFieldKey:otherFieldValue
  * otherFieldKey should equal the field names as they appear in your JIRA UI
  * note the sepratators, "|" and "," and ":"

it's recommended to:

* __use kubernetes secrets__ for your environment variables (note if you've got any whitespaces in your otherFieldKeys, you'll have to wrap the environment variable in quotes when you issue the kubectl create command)
* __create a new JIRA user__ for use by inspectr, that has limited access to a single project
* __use https__
* [obvious advice about passwords]
