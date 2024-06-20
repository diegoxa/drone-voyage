package plugin

import (
	"errors"
	"fmt"
	"os"

	appsV1 "k8s.io/api/apps/v1"
	batchV1 "k8s.io/api/batch/v1"
	coreV1 "k8s.io/api/core/v1"
	"sigs.k8s.io/yaml"

	"github.com/sirupsen/logrus"
)

type ManifestKind int

const (
	Deployment ManifestKind = iota
	Job
	CronJob
)

type manifest struct {
	Kind string
}

func (kind ManifestKind) String() string {
	m := [...]string{
		"Deployment",
		"Job",
		"CronJob",
	}
	if kind < Deployment || kind > CronJob {
		return "Unknown"
	}

	return m[kind]
}

type GetContainersInterface interface {
	GetContainers() ([]*coreV1.Container, error)
}

type DeploymentManifest appsV1.Deployment
type JobManifest batchV1.Job
type CronJobManifest batchV1.CronJob

func (m *DeploymentManifest) GetContainers() ([]*coreV1.Container, error) {
	var containers []*coreV1.Container
	if len(m.Spec.Template.Spec.Containers) == 0 {
		return containers, errors.New("no containers found")
	}

	for i := 0; i < len(m.Spec.Template.Spec.Containers); i++ {
		containers = append(containers, &m.Spec.Template.Spec.Containers[i])
	}
	return containers, nil
}

func (m *JobManifest) GetContainers() ([]*coreV1.Container, error) {
	var containers []*coreV1.Container
	if len(m.Spec.Template.Spec.Containers) == 0 {
		return containers, errors.New("no containers found")
	}

	for i := 0; i < len(m.Spec.Template.Spec.Containers); i++ {
		containers = append(containers, &m.Spec.Template.Spec.Containers[i])
	}
	return containers, nil
}

func (m *CronJobManifest) GetContainers() ([]*coreV1.Container, error) {
	var containers []*coreV1.Container
	if len(m.Spec.JobTemplate.Spec.Template.Spec.Containers) == 0 {
		return containers, errors.New("no containers found")
	}

	for i := 0; i < len(m.Spec.JobTemplate.Spec.Template.Spec.Containers); i++ {
		containers = append(containers, &m.Spec.JobTemplate.Spec.Template.Spec.Containers[i])
	}
	return containers, nil
}

func SetImage(m GetContainersInterface, image, filterByContainerName string) (bool, error) {
	c, err := m.GetContainers()
	if err != nil {
		return false, err
	}

	changeApplied := false
	for i := 0; i < len(c); i++ {
		if filterByContainerName != "" {
			if c[i].Name != filterByContainerName {
				logrus.Infof("filter by container: %s, skipping container: %s", filterByContainerName, c[i].Name)
				continue
			}
		}

		logrus.Debugln("found container in deployment file")
		logrus.Infof("  |- container: [%s]", c[i].Name)
		logrus.Infof("     |- current image: %s", c[i].Image)
		logrus.Infof("     |-     new image: %s", image)

		c[i].Image = image
		changeApplied = true
	}
	return changeApplied, nil
}

func UpdateImage(gitRepo *Repo, deploymentFiles []string, imageName, containerName string) bool {
	logrus.Debugln("updating images...")

	patch := false
	for _, manifestFile := range deploymentFiles {
		logrus.Debugf("updating deployment file %s\n", manifestFile)
		if patchContainerImage(manifestFile, gitRepo, imageName, containerName) {
			patch = true
		}
	}
	return patch
}

func patchContainerImage(deploymentFile string, repo *Repo, newImageName, containerName string) bool {
	manifestFile := fmt.Sprintf("%s/%s", repo.GetLocalDir(), deploymentFile)

	// Read the YAML file
	file, err := os.ReadFile(manifestFile)
	if err != nil {
		logrus.Fatalf("Error reading YAML file: %s", err)
	}

	//detect kind
	var m manifest
	err = yaml.Unmarshal(file, &m)
	if err != nil {
		logrus.Fatalf("Error parsing YAML file: %s", err)
	}

	var maf GetContainersInterface
	switch m.Kind {
	case Deployment.String():
		var f DeploymentManifest
		err = yaml.Unmarshal(file, &f)
		if err != nil {
			logrus.Fatalf("Error parsing YAML file: %s", err)
		}
		maf = &f
	case Job.String():
		var f JobManifest
		err = yaml.Unmarshal(file, &f)
		if err != nil {
			logrus.Fatalf("Error parsing YAML file: %s", err)
		}
		maf = &f
	case CronJob.String():
		var f CronJobManifest
		err = yaml.Unmarshal(file, &f)
		if err != nil {
			logrus.Fatalf("Error parsing YAML file: %s", err)
		}
		maf = &f
	default:
		logrus.Fatalf("manifest %s not supported", m.Kind)
	}

	logrus.Infof("manifest: `%s`", deploymentFile)
	success, setImageErr := SetImage(maf, newImageName, containerName)
	if setImageErr != nil {
		logrus.Fatalf("error setting image: %s", setImageErr)
	}

	if !success {
		return false
	}

	updatedYaml, errYalmMarshal := yaml.Marshal(maf)
	if errYalmMarshal != nil {
		logrus.Fatalf("Error writing YAML file: %s", err)
	}

	// Write the updated YAML back to the file
	err = os.WriteFile(manifestFile, updatedYaml, 0644)
	if err != nil {
		logrus.Fatalf("Error writing YAML file: %s", err)
	}

	logrus.Debugln("Successfully updated image attribute in the Kubernetes deployment manifest")
	return true
}
