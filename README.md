A plugin to update images within k8s manifest files on remote git repositories.

# Usage

The following settings changes this plugin's behavior.

* param1 (optional) does something.
* param2 (optional) does something different.

Below is an example `.drone.yml` that uses this plugin.

```yaml
kind: pipeline
name: default

steps:
- name: run diegoxa/voyage plugin
  image: diegoxa/voyage
  pull: if-not-exists
  settings:
    param1: foo
    param2: bar
```

# Building

Build the plugin binary:

```text
scripts/build.sh
```

Build the plugin image:

```text
docker build -t diegoxa/voyage -f docker/Dockerfile .
```

# Testing

Execute the plugin from your current working directory:

```text
docker run --rm \
  -e PLUGIN_GITHUB_REPO=git@github.com:moon/light.git \
  -e PLUGIN_GITHUB_SSH_KEY=$GIT_SSH_KEY 
  -e PLUGIN_IMAGE=my-docker/moon-light:v0.1 \
  -e PLUGIN_DEPLOYMENT_FILES=k8s/prod/deployment.yaml,k8s/prod/migration.yaml \
  -e PLUGIN_COMMIT_AUTHOR=John Doe \
  -e PLUGIN_COMMIT_EMAIL=jdoe@moon.com \
  -e PLUGIN_LOG_LEVEL=info \
  -e DRONE_COMMIT_SHA=8f51ad7884c5eb69c11d260a31da7a745e6b78e2 \
  -e DRONE_COMMIT_BRANCH=main \
  -e DRONE_BUILD_NUMBER=43 \
  -e DRONE_BUILD_STATUS=success \
  -w /drone/src \
diegoxa/voyage
```
