<p align="center">
<img src="./docs/voyage.svg" width="150" height="150">
</p>

Drone CI Plugin for updating image attribute in kubernetes deployment manifest files on a remote GitHub repository.

Ideal if you use ArgoCD for managing deployments.

# Usage

The following settings changes this plugin's behavior.

* github_repo (required) Github repository containing the k8s manifest files.
* github_ssh_key (required) Github private key.
* image (required) Docker image name, i.e.: diegoxa/drone-voyager:v1.0
* deployment_files (required) One or many comma separated files
* container_name (optional) The container name under deployment to apply the image change. Ideal when you have multiple containers in a manifest file, but you only need to update a specific container.
* commit_author (required) Author to be used on the commit.
* commit_email (required) Email to be used on the commit.
* log_level (optional) Log level. [info,debug]

Below is an example `.drone.yml` that uses this plugin.

```yaml
kind: pipeline
name: default

steps:
- name: run diegoxa/drone-voyage plugin
  image: diegoxa/drone-voyage
  pull: if-not-exists
  settings:
    github_repo: git@github.com:user/repo.git
    github_ssh_key:
      from_secret: deployment_ssh_key
    image: diegoxa/drone-voyage:v1-rc.2
    deployment_files: k8s/prod/deployment.yaml,k8s/prod/migration.yaml
    commit_author: Voyage
    commit_email: voyage@email.com
    log_level: info
```

# Building

Build the plugin binary:

```text
scripts/build.sh
```

Build the plugin image:

```text
docker build -t diegoxa/drone-voyage -f docker/Dockerfile .
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
  -w /drone/src \
  diegoxa/drone-voyage
```
