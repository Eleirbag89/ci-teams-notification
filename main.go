package main

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"mime"
	"net/http"
	"os"
	"path"
	"sort"
	"strconv"
	"strings"
	"time"
)

func main() {
	webhookURL := getEnvOrDefault("PLUGIN_WEBHOOK_URL", "")
	if webhookURL == "" {
		fmt.Println("Need to set MS Teams Webhook URL")
		os.Exit(1)
	}

	projectVersion := getProjectVersion()
	card := createTeamsCard(projectVersion)

	cardBytes, err := json.Marshal(card)
	if err != nil {
		fmt.Printf("Error creating card JSON: %v\n", err)
		os.Exit(1)
	}

	if getEnvOrDefault("PLUGIN_DEBUG", "false") == "true" {
		printDebugInfo(cardBytes)
	}

	printBuildInfo(projectVersion)
	sendCard(webhookURL, cardBytes)
}

func getProjectVersion() string {
	if tag := getEnvOrDefault("CI_COMMIT_TAG", ""); tag != "" {
		return tag
	}
	if sha := getEnvOrDefault("CI_COMMIT_SHA", ""); sha != "" {
		return sha[:7]
	}
	return ""
}

func createTeamsCard(projectVersion string) map[string]any {
	return map[string]any{
		"type": "message",
		"attachments": []map[string]any{
			{
				"contentType": "application/vnd.microsoft.card.adaptive",
				"contentUrl":  nil,
				"content": map[string]any{
					"$schema": "http://adaptivecards.io/schemas/adaptive-card.json",
					"type":    "AdaptiveCard",
					"version": "1.5",
					"body":    createCardBody(projectVersion),
					"actions": createCardActions(),
				},
			},
		},
	}
}

func getAvatarDataURI(avatarURL string) (string, error) {
	resp, err := http.Get(avatarURL)
	if err != nil {
		return "", fmt.Errorf("failed to download avatar: %w", err)
	}
	defer resp.Body.Close()

	imageData, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read avatar data: %w", err)
	}

	contentType := resp.Header.Get("Content-Type")
	ext := path.Ext(avatarURL)
	switch {
	case contentType != "":
		// Use content type from header
	case ext != "":
		contentType = mime.TypeByExtension(ext)
	default:
		contentType = http.DetectContentType(imageData)
	}

	return fmt.Sprintf("data:%s;base64,%s",
		contentType,
		base64.StdEncoding.EncodeToString(imageData),
	), nil
}

func createCardBody(projectVersion string) []map[string]any {
	status := getEnvOrDefault("DRONE_BUILD_STATUS", "")
	override_status := getEnvOrDefault("PLUGIN_STATUS", "")
	if override_status != "" {
		fmt.Printf("Overriding status to: %s\n", override_status)
		status = override_status
	}
	color := "good"
	title := "✔ Pipeline succeeded"
	if status == "failure" {
		color = "attention"
		title = "❌ Pipeline failed"
	}
	dateStr := time.Now().UTC().Format("2006-01-02T15:04:05Z")

	avatarURL := getEnvOrDefault("CI_COMMIT_AUTHOR_AVATAR", "")
	var avatarDataURI string
	if avatarURL != "" {
		if dataURI, err := getAvatarDataURI(avatarURL); err == nil {
			avatarDataURI = dataURI
		} else {
			fmt.Printf("Warning: Failed to process avatar image: %v\n", err)
			avatarDataURI = avatarURL
		}
	}

	body := createBaseBody(color, title, avatarDataURI, dateStr, projectVersion)

	if variables := getEnvOrDefault("PLUGIN_VARIABLES", ""); variables != "" {
		body = appendVariablesTable(body, variables)
	}

	return body
}

func createBaseBody(color, title, avatarDataURI, dateStr, projectVersion string) []map[string]any {
	body := []map[string]any{
		{
			"type":    "Container",
			"bleed":   true,
			"spacing": "None",
			"style":   color,
			"items": []map[string]any{
				{
					"type":   "TextBlock",
					"text":   title,
					"weight": "bolder",
					"size":   "medium",
					"color":  color,
				},
				createAuthorSection(avatarDataURI, dateStr),
			},
		},
	}

	if facts := createFactsSection(projectVersion); facts != nil {
		body = append(body, facts)
	}

	return body
}

func createAuthorSection(avatarDataURI, dateStr string) map[string]any {
	return map[string]any{
		"type": "ColumnSet",
		"columns": []map[string]any{
			{
				"type":  "Column",
				"width": "auto",
				"items": []map[string]any{
					{
						"type":  "Image",
						"url":   avatarDataURI,
						"size":  "small",
						"style": "Person",
					},
				},
			},
			{
				"type":  "Column",
				"width": "stretch",
				"items": []map[string]any{
					{
						"type":   "TextBlock",
						"text":   "@" + getEnvOrDefault("CI_COMMIT_AUTHOR", ""),
						"weight": "bolder",
						"wrap":   true,
					},
					{
						"type":     "TextBlock",
						"spacing":  "None",
						"text":     fmt.Sprintf("{{DATE(%s, SHORT)}} at {{TIME(%s)}}", dateStr, dateStr),
						"isSubtle": true,
						"wrap":     true,
					},
				},
			},
		},
	}
}

func createFactsSection(projectVersion string) map[string]any {
	// Define available facts
	allFacts := map[string]map[string]string{
		"project": {
			"title": "Project:",
			"value": getEnvOrDefault("CI_REPO", ""),
		},
		"message": {
			"title": "Message:",
			"value": strings.Split(getEnvOrDefault("CI_COMMIT_MESSAGE", ""), "\n")[0],
		},
		"version": {
			"title": "Version:",
			"value": projectVersion,
		},
	}

	// Get requested facts
	var facts []map[string]string
	requestedFacts := getEnvOrDefault("PLUGIN_FACTS", "")
	if requestedFacts == "" {
		// If no facts specified, show all
		for _, fact := range allFacts {
			facts = append(facts, fact)
		}
	} else {
		// Show only requested facts
		for _, name := range strings.Split(requestedFacts, ",") {
			name = strings.TrimSpace(name)
			if fact, exists := allFacts[name]; exists {
				facts = append(facts, fact)
			}
		}
	}

	// Return nil if no facts to show
	if len(facts) == 0 {
		return nil
	}

	return map[string]any{
		"type": "Container",
		"items": []map[string]any{
			{
				"type":  "FactSet",
				"facts": facts,
			},
		},
	}
}

func appendVariablesTable(body []map[string]any, variables string) []map[string]any {
	body = append(body, map[string]any{
		"type":   "TextBlock",
		"text":   "Variables:",
		"weight": "bolder",
		"wrap":   true,
	})

	var rows []map[string]any
	for _, varName := range strings.Split(variables, ",") {
		varName = strings.TrimSpace(varName)
		rows = append(rows, createTableRow(varName, getEnvOrDefault(varName, "")))
	}

	body = append(body, map[string]any{
		"type": "Table",
		"columns": []map[string]any{
			{"width": 1},
			{"width": 2},
		},
		"spacing":           "Small",
		"showGridLines":     false,
		"firstRowAsHeaders": false,
		"rows":              rows,
	})

	return body
}

func createTableRow(name, value string) map[string]any {
	return map[string]any{
		"type": "TableRow",
		"cells": []map[string]any{
			createTableCell(name),
			createTableCell(value),
		},
		"style": "default",
	}
}

func createTableCell(text string) map[string]any {
	return map[string]any{
		"type": "TableCell",
		"items": []map[string]any{
			{
				"type":     "TextBlock",
				"text":     text,
				"wrap":     true,
				"weight":   "Default",
				"fontType": "Monospace",
			},
		},
	}
}

func createCardActions() []map[string]any {
	// Define available actions
	allActions := map[string]any{
		"pipeline": map[string]any{
			"type":  "Action.OpenUrl",
			"title": "View Pipeline",
			"url":   getEnvOrDefault("CI_PIPELINE_URL", ""),
		},
	}

	// Add commit/release action
	actionURL := getEnvOrDefault("CI_PIPELINE_FORGE_URL", "")

	if tag := getEnvOrDefault("CI_COMMIT_TAG", ""); tag != "" {
		actionURL = fmt.Sprintf("%s/releases/tag/%s", getEnvOrDefault("CI_REPO_URL", ""), tag)
		allActions["release"] = map[string]any{
			"type":  "Action.OpenUrl",
			"title": "View Release",
			"url":   actionURL,
		}
	} else {
		allActions["commit"] = map[string]any{
			"type":  "Action.OpenUrl",
			"title": "View Commit",
			"url":   actionURL,
		}
	}

	// Get requested buttons
	var actions []map[string]any
	requestedButtons := getEnvOrDefault("PLUGIN_BUTTONS", "")

	if requestedButtons == "" {
		// If no buttons specified, show all with pipeline first
		if pipeline, exists := allActions["pipeline"]; exists {
			actions = append(actions, pipeline.(map[string]any))
		}
		for name, action := range allActions {
			if name != "pipeline" {
				actions = append(actions, action.(map[string]any))
			}
		}
	} else {
		// Show buttons in the order specified in PLUGIN_BUTTONS
		for _, name := range strings.Split(requestedButtons, ",") {
			name = strings.TrimSpace(name)
			if action, exists := allActions[name]; exists {
				actions = append(actions, action.(map[string]any))
			}
		}
	}

	return actions
}

func printBuildInfo(projectVersion string) {
	fmt.Println("\nBuild Info:")
	fmt.Printf(" PROJECT: %s\n", getEnvOrDefault("CI_REPO", ""))
	fmt.Printf(" VERSION: %s\n", projectVersion)
	fmt.Printf(" STATUS:  %s\n", getEnvOrDefault("DRONE_BUILD_STATUS", ""))
	fmt.Printf(" DATE:    %s\n", time.Now().UTC().Format(time.RFC3339))
}

func sendCard(webhookURL string, cardBytes []byte) {
	fmt.Println("\nSending to Microsoft Teams...")

	resp, err := http.Post(webhookURL, "application/json", bytes.NewReader(cardBytes))
	if err != nil {
		fmt.Printf("Error sending to Teams: %v\n", err)
		os.Exit(1)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		fmt.Printf("Error response from Teams: %s\n", string(body))
		os.Exit(1)
	}

	fmt.Println("Done!")
}

// getEnvOrDefault returns environment variable value or default if not set
func getEnvOrDefault(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

// parseTimestamp parses unix timestamp from string
func parseTimestamp(timestamp string) time.Time {
	if timestamp == "" {
		return time.Now()
	}
	ts, err := strconv.ParseInt(timestamp, 10, 64)
	if err != nil {
		return time.Now()
	}
	return time.Unix(ts, 0)
}

func printDebugInfo(cardBytes []byte) {
	fmt.Println("\n** DEBUG ENABLED **")
	fmt.Println("\nEnvironment Variables:")

	// Get and sort environment variables
	envVars := os.Environ()
	sort.Strings(envVars)

	// Print sorted variables
	for _, env := range envVars {
		// Split into key=value
		parts := strings.SplitN(env, "=", 2)
		if len(parts) == 2 {
			// Print in aligned format
			fmt.Printf(" %-30s = %s\n", parts[0], parts[1])
		}
	}

	fmt.Println("\nCard JSON:")
	fmt.Println(string(cardBytes))
}
