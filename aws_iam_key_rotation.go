package main

import (
	"bufio"
	"flag"
	"fmt"
	"os"
	"os/user"
	"path/filepath"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/iam"
)

var (
	createFlag   bool
	accessKey    string
	updateStatus string
	deleteKey    string
	listFlag     bool
	rotateFlag   bool
	maxKeyAge    = 30 // Key age threshold in days for rotation
)

func init() {
	flag.BoolVar(&createFlag, "create", false, "create access key")
	flag.StringVar(&accessKey, "accessKey", "", "access key ID, can be used only with -updateStatus flags")
	flag.StringVar(&updateStatus, "updateStatus", "", "update access key status: active|inactive; requires -accessKey flag")
	flag.StringVar(&deleteKey, "delete", "", "delete access key; expected input: <keyID>")
	flag.BoolVar(&listFlag, "list", false, "list access key ID, status, creation date")
	flag.BoolVar(&rotateFlag, "rotate", false, "rotate access key if older than 30 days")
}

func main() {
	flag.Parse()

	sess, err := session.NewSession(&aws.Config{
		//	Region: aws.String("us-east-1"),
	})
	if err != nil {
		fmt.Println("Error creating AWS session:", err)
		return
	}

	client := iam.New(sess)

	switch {
	case createFlag:
		createAccessKey(client)
	case accessKey != "" && updateStatus != "":
		updateAccessKey(client, accessKey, updateStatus)
	case deleteKey != "":
		deleteAccessKey(client, deleteKey)
	case listFlag:
		listAccessKeys(client)
	case rotateFlag:
		rotateAccessKeys(client)
	default:
		fmt.Println("Invalid command. Use -h for help.")
	}
}

func createAccessKey(client *iam.IAM) (string, error) {
	// Create a new access key
	result, err := client.CreateAccessKey(&iam.CreateAccessKeyInput{})
	if err != nil {
		fmt.Println("Error creating access key:", err)
		return "", err
	}

	fmt.Printf("New Key:        %s\n", *result.AccessKey.AccessKeyId)
	fmt.Printf("Secret:         %s\n", *result.AccessKey.SecretAccessKey)

	// Prompt to update the credentials file
	reader := bufio.NewReader(os.Stdin)
	fmt.Print("\nUpdate credentials file with new key (y/n)? Warning: File contents will be overwritten! ")
	response, _ := reader.ReadString('\n')
	response = strings.TrimSpace(response)

	if strings.ToLower(response) != "y" {
		fmt.Println("\nYou answered 'n' or provided incorrect input, bye!")
		return *result.AccessKey.AccessKeyId, nil
	}

	usr, err := user.Current()
	if err != nil {
		fmt.Println("Error getting current user:", err)
		return "", err
	}
	credPath := filepath.Join(usr.HomeDir, ".aws", "credentials")

	// Open the existing credentials file for reading
	file, err := os.Open(credPath)
	if err != nil {
		fmt.Println("Error opening credentials file for reading:", err)
		return "", err
	}

	var lines []string
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := scanner.Text()
		if strings.Contains(line, "aws_access_key_id") {
			line = fmt.Sprintf("aws_access_key_id = %s", *result.AccessKey.AccessKeyId)
		} else if strings.Contains(line, "aws_secret_access_key") {
			line = fmt.Sprintf("aws_secret_access_key = %s", *result.AccessKey.SecretAccessKey)
		}
		lines = append(lines, line)
	}

	if err := scanner.Err(); err != nil {
		fmt.Println("Error reading credentials file:", err)
		return "", err
	}

	file.Close()

	// Write the updated content back to the credentials file
	updatedContent := strings.Join(lines, "\n")
	err = os.WriteFile(credPath, []byte(updatedContent), 0644)
	if err != nil {
		fmt.Println("Error writing updated credentials file:", err)
		return "", err
	}

	fmt.Printf("\nUpdated credentials file at %s\n", credPath)
	return *result.AccessKey.AccessKeyId, nil
}

func updateAccessKey(client *iam.IAM, accessKeyId, status string) {
	// Ensure status is properly capitalized
	var capitalizedStatus string
	if strings.ToLower(status) == "active" {
		capitalizedStatus = "Active"
	} else if strings.ToLower(status) == "inactive" {
		capitalizedStatus = "Inactive"
	} else {
		fmt.Println("Invalid status provided. Please use 'active' or 'inactive'.")
		return
	}

	input := &iam.UpdateAccessKeyInput{
		AccessKeyId: aws.String(accessKeyId),
		Status:      aws.String(capitalizedStatus),
	}

	_, err := client.UpdateAccessKey(input)
	if err != nil {
		fmt.Printf("Error updating access key: %s\n", err)
		return
	}

	fmt.Printf("Access key %s updated to %s status\n", accessKeyId, capitalizedStatus)
}

func deleteAccessKey(client *iam.IAM, accessKeyId string) {
	_, err := client.DeleteAccessKey(&iam.DeleteAccessKeyInput{
		AccessKeyId: aws.String(accessKeyId),
	})
	if err != nil {
		fmt.Printf("Error deleting access key: %s\n", err)
		return
	}

	fmt.Printf("Access key %s has been deleted!\n", accessKeyId)
}

func listAccessKeys(client *iam.IAM) {
	result, err := client.ListAccessKeys(&iam.ListAccessKeysInput{})
	if err != nil {
		fmt.Println("Error listing access keys:", err)
		return
	}

	for _, key := range result.AccessKeyMetadata {
		fmt.Printf("Key:        %s\n", *key.AccessKeyId)
		fmt.Printf("Status:     %s\n", *key.Status)
		fmt.Printf("Created:    %s\n\n", key.CreateDate)
	}
}

func rotateAccessKeys(client *iam.IAM) {
	keys, err := client.ListAccessKeys(&iam.ListAccessKeysInput{})
	if err != nil {
		fmt.Println("Error listing access keys:", err)
		return
	}

	var activeKeys, inactiveKeys []*iam.AccessKeyMetadata
	for _, key := range keys.AccessKeyMetadata {
		if *key.Status == "Active" {
			activeKeys = append(activeKeys, key)
		} else {
			inactiveKeys = append(inactiveKeys, key)
		}
	}

	if len(activeKeys) > 1 {
		fmt.Println("User has 2 API keys and both are active. Please delete 1 key or make it inactive first to proceed.")
		return
	}

	if len(activeKeys) == 1 && len(inactiveKeys) == 1 {
		// If there is one active and one inactive key, delete the inactive key first
		fmt.Printf("Deleting old inactive key: %s\n", *inactiveKeys[0].AccessKeyId)
		deleteAccessKey(client, *inactiveKeys[0].AccessKeyId)
	}

	activeKeyID, err := getActiveKeyIDFromConfig()
	if err != nil {
		fmt.Printf("Error retrieving active key ID: %s\n", err)
		return
	}

	// If there's only one active key, check its age and proceed with rotation if necessary
	if len(activeKeys) == 1 && (time.Since(*activeKeys[0].CreateDate).Hours()/24) > float64(maxKeyAge) {
		// Found the active key is old, rotate it
		fmt.Println("Rotating active key...")
		_, err := createAccessKey(client) // Assume this returns the new AccessKeyID or an error
		if err != nil {
			fmt.Printf("Error creating access key: %s\n", err)
			return
		}
		// After successfully creating a new key, delete the old one
		fmt.Printf("Deleting old active key: %s\n", activeKeyID)
		deleteAccessKey(client, activeKeyID)
	} else if len(activeKeys) == 1 {
		fmt.Println("The active key is within the age threshold; no rotation necessary.")
	}
}
func getActiveKeyIDFromConfig() (string, error) {
	usr, _ := user.Current()
	configPath := filepath.Join(usr.HomeDir, ".aws", "credentials")
	file, err := os.Open(configPath)
	if err != nil {
		return "", err
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	activeKeyID := ""
	for scanner.Scan() {
		line := scanner.Text()
		if strings.Contains(line, "aws_access_key_id") {
			parts := strings.Split(line, "=")
			if len(parts) == 2 {
				activeKeyID = strings.TrimSpace(parts[1])
				break
			}
		}
	}

	if err := scanner.Err(); err != nil {
		return "", err
	}

	return activeKeyID, nil
}
