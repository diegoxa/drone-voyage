package plugin

import (
	"fmt"
	"os"

	v1 "k8s.io/api/apps/v1"
	"sigs.k8s.io/yaml"

	"github.com/sirupsen/logrus"
)

func UpdateImage(gitRepo *Repo, deploymentFiles []string, imageName, containerName string) bool {
	logrus.Debugln("updating images...")

	patch := false
	for _, deploymentFile := range deploymentFiles {
		logrus.Debugf("updating deployment file %s\n", deploymentFile)
		if patchContainerImage(deploymentFile, gitRepo, imageName, containerName) {
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

	// Unmarshal the YAML to a Deployment struct
	var deployment v1.Deployment
	err = yaml.Unmarshal(file, &deployment)
	if err != nil {
		logrus.Fatalf("Error parsing YAML file: %s", err)
	}

	patch := false
	// Update the image field
	// Be sure to verify that at least one container specification exists
	if len(deployment.Spec.Template.Spec.Containers) > 0 {
		for i := 0; i < len(deployment.Spec.Template.Spec.Containers); i++ {
			c := &deployment.Spec.Template.Spec.Containers[i]
			if containerName != "" {
				if c.Name != containerName {
					logrus.Infof("filter by container: %s, skipping container: %s", containerName, c.Name)
					continue
				}
			}
			logrus.Debugln("found container in deployment file")
			logrus.Infof("manifest: `%s`", deploymentFile)
			logrus.Infof("  |- container: [%s]", c.Name)
			logrus.Infof("     |- current image: %s", c.Image)
			logrus.Infof("     |-     new image: %s", newImageName)
			c.Image = newImageName
			patch = true
		}
	}

	if !patch {
		return false
	}

	// Marshal the deployment back to YAML
	updatedYaml, err := yaml.Marshal(&deployment)
	if err != nil {
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
