# inspectr

this is a binary that, when run in a k8s cluster, gets details of pods in the same cluster (via the k8s master/API), and alerts if certain conditions are met.

currently the only condition that's ever met is "are there any upgrades to the image being run in pod X?"


## result grouping

results are unique by cluster/namespace/image


## running frequency

there is a daily 'scheduled alert window', within which all results obtained will be outputted

outside of this window, the process runs every minute

if any errors are encountered, the process won't run again for 5 minutes.


## Slack alerts

the binary needs to know the webhook id that you want the alerts going to

it looks for this id in the environment variable "INSPECTR_SLACK_WEBHOOK_ID"

this id is the string that comes after "https://hooks.slack.com/services/" in the webhook URL

if that's not set, the binary still runs, you just (obviously) won't see any alerts in your Slack channel. Inspectr results are still logged via glog.


## alert cache

to prevent noise, the binary keeps a cache of the clusters/images that an alert has been produced for

if a cluster/image appears in the cache, it won't get alerted on again until the 'scheduled alert window'

when the binary first runs, the cache is empty, so essentially you'll get a full result alert every time a pod starts/restarts
