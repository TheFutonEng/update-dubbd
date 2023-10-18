package main

import (
    "fmt"
    "io/ioutil"
    "os"
    "os/exec"
    "os/signal"
    "regexp"
    "syscall"
    "time"

    "github.com/slack-go/slack"
)

func main() {
    // Read the Slack API token from an environment variable
    slackAPIToken := os.Getenv("SLACK_API_TOKEN")

    if slackAPIToken == "" {
        fmt.Println("SLACK_API_TOKEN environment variable is not set. Please set it before running the program.")
        os.Exit(1)
    }

    // Define the user whose messages you want to scrape
    userName := "uds-new-release"

    // Define the channel name
    channelName := "public-uds"

    // Define the regular expression pattern to match the version format
    versionPattern := regexp.MustCompile(`DUBBD v(\d+\.\d+\.\d+)`)

    // Create a signal channel to handle graceful exit
    sigChannel := make(chan os.Signal, 1)
    signal.Notify(sigChannel, syscall.SIGINT, syscall.SIGTERM)

    // Define the scraping interval (4 hours)
    scrapingInterval := 4 * time.Hour

    // Path to the file where the latest version is stored
    versionFilePath := "latest_version.txt"

    for {
        // Read the latest version from the file
        currentVersion, err := readVersionFromFile(versionFilePath)
        if err != nil {
            fmt.Printf("Error reading the latest version: %s\n", err)
            os.Exit(1)
        }

        // Find the public-uds channel (or any desired channel)
        channels, err := api.GetConversations(&slack.GetConversationsParameters{ExcludeArchived: true})
        if err != nil {
            fmt.Printf("Error fetching channels: %s\n", err)
            os.Exit(1)
        }

        var channelID string
        for _, channel := range channels {
            if channel.Name == channelName {
                channelID = channel.ID
                break
            }
        }

        if channelID == "" {
            fmt.Printf("Channel '%s' not found.\n", channelName)
            os.Exit(1)
        }

        // Fetch the user's messages from the public-uds channel
        messages, _, err := api.GetConversationHistory(&slack.GetConversationHistoryParameters{
            ChannelID: channelID,
        })
        if err != nil {
            fmt.Printf("Error fetching conversation history: %s\n", err)
            os.Exit(1)
        }

        // Initialize a variable to store the extracted version
        var extractedVersion string

        for _, message := range messages {
            if message.User == userName && message.Text != "" {
                // Check if the message contains the version number
                match := versionPattern.FindStringSubmatch(message.Text)
                if len(match) == 2 {
                    extractedVersion = match[1]
                    break
                }
            }
        }

        if extractedVersion != "" {
            fmt.Printf("Extracted version: %s\n", extractedVersion)

            // Compare the extracted version with the current version
            if extractedVersion != currentVersion {
                // Update the latest version in the file
                err := writeVersionToFile(versionFilePath, extractedVersion)
                if err != nil {
                    fmt.Printf("Error writing the latest version to file: %s\n", err)
                }

                // Run the command with the extracted version
                command := fmt.Sprintf("zarf package deploy oci://ghcr.io/defenseunicorns/packages/dubbd-k3d:%s-amd64 --oci-concurrency=15 --confirm", extractedVersion)
                cmd := exec.Command("/bin/sh", "-c", command)

                cmd.Stdout = os.Stdout
                cmd.Stderr = os.Stderr

                err := cmd.Run()
                if err != nil {
                    fmt.Printf("Error running the command: %s\n", err)
                }
            } else {
                fmt.Println("No new version found.")
            }
        } else {
            fmt.Println("Version not found in user's messages.")
        }

        // Sleep for the scraping interval
        time.Sleep(scrapingInterval)

        select {
        case sig := <-sigChannel:
            fmt.Printf("Received signal %s. Exiting...\n", sig)
            os.Exit(0)
        default:
            // Continue scraping
        }
    }
}

func readVersionFromFile(filePath string) (string, error) {
    content, err := ioutil.ReadFile(filePath)
    if err != nil {
        return "", err
    }
    return string(content), nil
}

func writeVersionToFile(filePath, version string) error {
    return ioutil.WriteFile(filePath, []byte(version), 0644)
}
