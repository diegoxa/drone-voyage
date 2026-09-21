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

func SetImage(m GetContainersInterface, image string, filterByContainerNames []string) (bool, error) {
	c, err := m.GetContainers()
	if err != nil {
		return false, err
	}

	changeApplied := false
	for i := 0; i < len(c); i++ {
		if len(filterByContainerNames) > 0 {
			found := false
			for _, name := range filterByContainerNames {
				if c[i].Name == name {
					found = true
					break
				}
			}
			if !found {
				logrus.Infof("filter by containers: %v, skipping container: %s", filterByContainerNames, c[i].Name)
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

func UpdateImage(gitRepo *Repo, deploymentFiles []string, imageName string, containerNames []string) (bool, error) {
	logrus.Debugln("updating images...")

	patch := false
	for _, manifestFile := range deploymentFiles {
		logrus.Debugf("updating deployment file %s\n", manifestFile)
		applied, err := patchContainerImage(manifestFile, gitRepo, imageName, containerNames)
		if err != nil {
			return false, err
		}
		if applied {
			patch = true
		}
	}
	return patch, nil
}

func patchContainerImage(deploymentFile string, repo *Repo, newImageName string, containerNames []string) (bool, error) {
	manifestFile := fmt.Sprintf("%s/%s", repo.GetLocalDir(), deploymentFile)

	// Read the YAML file
	file, err := os.ReadFile(manifestFile)
	if err != nil {
		return false, fmt.Errorf("error reading YAML file %s: %w", manifestFile, err)
	}

	//detect kind
	var m manifest
	err = yaml.Unmarshal(file, &m)
	if err != nil {
		return false, fmt.Errorf("error parsing YAML file %s: %w", manifestFile, err)
	}

	var maf GetContainersInterface
	switch m.Kind {
	case Deployment.String():
		var f DeploymentManifest
		err = yaml.Unmarshal(file, &f)
		if err != nil {
			return false, fmt.Errorf("error parsing YAML file %s: %w", manifestFile, err)
		}
		maf = &f
	case Job.String():
		var f JobManifest
		err = yaml.Unmarshal(file, &f)
		if err != nil {
			return false, fmt.Errorf("error parsing YAML file %s: %w", manifestFile, err)
		}
		maf = &f
	case CronJob.String():
		var f CronJobManifest
		err = yaml.Unmarshal(file, &f)
		if err != nil {
			return false, fmt.Errorf("error parsing YAML file %s: %w", manifestFile, err)
		}
		maf = &f
	default:
		return false, fmt.Errorf("manifest %s not supported", m.Kind)
	}

	logrus.Infof("manifest: `%s`", deploymentFile)
	success, err := SetImage(maf, newImageName, containerNames)
	if err != nil {
		return false, fmt.Errorf("error setting image: %w", err)
	}

	if !success {
		return false, nil
	}

	updatedYaml, err := yaml.Marshal(maf)
	if err != nil {
		return false, fmt.Errorf("error writing YAML file %s: %w", manifestFile, err)
	}

	// Write the updated YAML back to the file
	if err := os.WriteFile(manifestFile, updatedYaml, 0644); err != nil {
		return false, fmt.Errorf("error writing YAML file %s: %w", manifestFile, err)
	}

	logrus.Debugln("Successfully updated image attribute in the Kubernetes deployment manifest")
	return true, nil
}
