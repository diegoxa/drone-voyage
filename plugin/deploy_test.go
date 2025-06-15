package plugin

import (
	"fmt"
	"os"
	"sigs.k8s.io/yaml"
	"testing"
)

var deploymentYaml = `
apiVersion: apps/v1
kind: Deployment
metadata:
  name: nginx-deployment
spec:
  replicas: 3
  selector:
    matchLabels:
      app: nginx
  template:
    metadata:
      labels:
        app: nginx
    spec:
      containers:
      - name: nginx
        image: nginx:1.16.0
        ports:
        - containerPort: 80
      - name: test
        image: test:1
`
var jobYaml = `
apiVersion: batch/v1
kind: Job
metadata:
  name: example-job
spec:
  template:
    metadata:
      name: example-job
    spec:
      containers:
      - name: job-container
        image: busybox
        command: ["sh", "-c", "echo Hello Kubernetes! && sleep 30"]
      restartPolicy: Never
`

var cronjobYaml = `
apiVersion: batch/v1beta1
kind: CronJob
metadata:
  name: hello
spec:
  schedule: "*/1 * * * *"
  jobTemplate:
    spec:
      template:
        spec:
          containers:
          - name: hello
            image: busybox
            args:
            - /bin/sh
            - -c
            - date; echo Hello from the Kubernetes cluster
          restartPolicy: OnFailure
`

func TestManifestDetection(t *testing.T) {
	m := [...]ManifestKind{
		Deployment,
		Job,
		CronJob,
	}

	var f manifest
	for _, mKind := range m {
		_ = yaml.Unmarshal([]byte(fmt.Sprintf("apiVersion: apps/v1\nkind: %s", mKind.String())), &f)
		if mKind.String() != f.Kind {
			t.Errorf("could not identify the manifest type %s", mKind.String())
		}
	}
}

func TestGetContainers(t *testing.T) {
	y := deploymentYaml
	var m DeploymentManifest

	e := yaml.Unmarshal([]byte(y), &m)
	if e != nil {
		t.Error(e)
	}

	c, _ := m.GetContainers()
	if c[0].Name != "nginx" {
		t.Error("container name doesn't match")
	}
	if c[0].Image != "nginx:1.16.0" {
		t.Error("container image doesn't match")
	}
	if c[1].Name != "test" {
		t.Error("container name doesn't match")
	}
	if c[1].Image != "test:1" {
		t.Error("container image doesn't match")
	}

	j := jobYaml
	var job JobManifest
	e = yaml.Unmarshal([]byte(j), &job)
	if e != nil {
		t.Error(e)
	}

	c, _ = job.GetContainers()
	if c[0].Name != "job-container" {
		t.Error("container name doesn't match")
	}
}

func TestSetImage(t *testing.T) {
	y := deploymentYaml
	var m DeploymentManifest

	e := yaml.Unmarshal([]byte(y), &m)
	if e != nil {
		t.Error(e)
	}

	// Test with empty filter (should update all containers)
	success, err := SetImage(&m, "new-image:2", []string{})
	if err != nil {
		t.Error(err)
	}
	if !success {
		t.Error("change was not applied")
	}

	if m.Spec.Template.Spec.Containers[0].Image != "new-image:2" {
		t.Error("image was not updated")
	}

	// Test with single container filter
	success, err = SetImage(&m, "new-image:3", []string{"test"})
	if err != nil {
		t.Error(err)
	}

	if !success {
		t.Error("change was not applied")
	}

	if m.Spec.Template.Spec.Containers[0].Image != "new-image:2" {
		t.Error("image was not updated")
	}

	if m.Spec.Template.Spec.Containers[1].Image != "new-image:3" {
		t.Error("image was not updated")
	}

	// Test with non-existent container
	success, err = SetImage(&m, "new-image:10", []string{"not-exist"})
	if err != nil {
		t.Error(err)
	}

	if success {
		t.Error("container was not found yet a change was applied")
	}

	// Test with multiple container filters
	success, err = SetImage(&m, "new-image:4", []string{"nginx", "test"})
	if err != nil {
		t.Error(err)
	}

	if !success {
		t.Error("change was not applied")
	}

	if m.Spec.Template.Spec.Containers[0].Image != "new-image:4" {
		t.Error("image was not updated for nginx container")
	}

	if m.Spec.Template.Spec.Containers[1].Image != "new-image:4" {
		t.Error("image was not updated for test container")
	}

	j := jobYaml
	var job JobManifest
	e = yaml.Unmarshal([]byte(j), &job)
	if e != nil {
		t.Error(e)
	}

	// Test with empty filter on job
	success, err = SetImage(&job, "new-image:2", []string{})
	if err != nil {
		t.Error(err)
	}
	if !success {
		t.Error("change was not applied")
	}

	if job.Spec.Template.Spec.Containers[0].Image != "new-image:2" {
		t.Error("image was not updated")
	}

	// Test with non-existent container on job
	success, err = SetImage(&job, "new-image:2", []string{"other-container"})
	if err != nil {
		t.Error(err)
	}
	if success {
		t.Error("change was applied incorrectly")
	}

	// Test with cronjob
	cj := cronjobYaml
	var cronjob CronJobManifest
	e = yaml.Unmarshal([]byte(cj), &cronjob)
	if e != nil {
		t.Error(e)
	}

	// Test with empty filter on cronjob
	success, err = SetImage(&cronjob, "new-image:3", []string{})
	if err != nil {
		t.Error(err)
	}
	if !success {
		t.Error("change was not applied")
	}

	if cronjob.Spec.JobTemplate.Spec.Template.Spec.Containers[0].Image != "new-image:3" {
		t.Error("image was not updated in cronjob")
	}

	// Test with specific container name on cronjob
	success, err = SetImage(&cronjob, "new-image:4", []string{"hello"})
	if err != nil {
		t.Error(err)
	}
	if !success {
		t.Error("change was not applied")
	}

	if cronjob.Spec.JobTemplate.Spec.Template.Spec.Containers[0].Image != "new-image:4" {
		t.Error("image was not updated in cronjob with container filter")
	}

	// Test with non-existent container on cronjob
	success, err = SetImage(&cronjob, "new-image:5", []string{"non-existent"})
	if err != nil {
		t.Error(err)
	}
	if success {
		t.Error("change was applied incorrectly for non-existent container in cronjob")
	}
}

func TestPatchContainerImage(t *testing.T) {
	folder := generateRandomString(5)
	repo := &Repo{
		localDir: "/tmp/" + folder,
	}

	repo.Cleanup()

	_ = os.Mkdir("/tmp/"+folder, 0755)
	_ = os.WriteFile("/tmp/"+folder+"/test1.yaml", []byte(deploymentYaml), 0644)

	success := patchContainerImage("test1.yaml", repo, "a:1", []string{"nginx"})

	if !success {
		t.Error("patchContainerImage failed")
	}

	updatedFile, _ := os.ReadFile("/tmp/" + folder + "/test1.yaml")

	var depManifestUpdated DeploymentManifest
	_ = yaml.Unmarshal(updatedFile, &depManifestUpdated)
	if depManifestUpdated.Spec.Template.Spec.Containers[0].Image != "a:1" {
		t.Error("deployment was not updated correctly")
	}

	if depManifestUpdated.Spec.Template.Spec.Containers[1].Image != "test:1" {
		t.Error("deployment was updated incorrectly")
	}

	repo.Cleanup()
}

func TestUpdateImage(t *testing.T) {
	folder := generateRandomString(5)
	repo := &Repo{
		localDir: "/tmp/" + folder,
	}
	repo.Cleanup()
	_ = os.Mkdir("/tmp/"+folder, 0755)
	_ = os.WriteFile("/tmp/"+folder+"/test1.yaml", []byte(deploymentYaml), 0644)
	_ = os.WriteFile("/tmp/"+folder+"/test2.yaml", []byte(jobYaml), 0644)

	success := UpdateImage(repo, []string{"test1.yaml", "test2.yaml"}, "new-image:v123", []string{})
	if !success {
		t.Error("no updates were made")
	}

	updatedFile, _ := os.ReadFile("/tmp/" + folder + "/test1.yaml")
	var depManifestUpdated DeploymentManifest
	_ = yaml.Unmarshal(updatedFile, &depManifestUpdated)
	if depManifestUpdated.Spec.Template.Spec.Containers[0].Image != "new-image:v123" {
		t.Error("deployment was not updated correctly")
	}
	if depManifestUpdated.Spec.Template.Spec.Containers[1].Image != "new-image:v123" {
		t.Error("deployment was not updated correctly")
	}

	updatedFile2, _ := os.ReadFile("/tmp/" + folder + "/test2.yaml")
	var j JobManifest
	_ = yaml.Unmarshal(updatedFile2, &j)
	if j.Spec.Template.Spec.Containers[0].Image != "new-image:v123" {
		t.Error("deployment was not updated correctly")
	}

	repo.Cleanup()
}
