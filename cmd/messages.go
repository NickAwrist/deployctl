package cmd

import "fmt"

const (
	maskedValue  = "*****"
	noneValue    = "none"
	unknownValue = "unknown"
)

func deploymentSuccess(deploymentName string, action string) string {
	if deploymentName == "" {
		return fmt.Sprintf("Deployment %s successfully", action)
	}
	return fmt.Sprintf("Deployment %s %s successfully", deploymentName, action)
}

func envChangeSuccess(action string, count int, preposition string, deploymentName string) string {
	return fmt.Sprintf("%s %s %s %s", action, countPhrase(count, "env variable"), preposition, deploymentName)
}

func envFileImportedSuccess(deploymentName string) string {
	return fmt.Sprintf("Imported env file for %s", deploymentName)
}

func countPhrase(count int, singular string) string {
	if count == 1 {
		return fmt.Sprintf("1 %s", singular)
	}
	return fmt.Sprintf("%d %ss", count, singular)
}
