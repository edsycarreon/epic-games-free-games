package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

// DiscordEmbed represents a Discord embed object
type DiscordEmbed struct {
	Title       string                 `json:"title,omitempty"`
	Description string                 `json:"description,omitempty"`
	URL         string                 `json:"url,omitempty"`
	Color       int                    `json:"color,omitempty"`
	Timestamp   string                 `json:"timestamp,omitempty"`
	Fields      []DiscordEmbedField    `json:"fields,omitempty"`
	Thumbnail   *DiscordEmbedThumbnail `json:"thumbnail,omitempty"`
	Footer      *DiscordEmbedFooter    `json:"footer,omitempty"`
}

// DiscordEmbedField represents a field in a Discord embed
type DiscordEmbedField struct {
	Name   string `json:"name"`
	Value  string `json:"value"`
	Inline bool   `json:"inline,omitempty"`
}

// DiscordEmbedThumbnail represents a thumbnail in a Discord embed
type DiscordEmbedThumbnail struct {
	URL string `json:"url"`
}

// DiscordEmbedFooter represents a footer in a Discord embed
type DiscordEmbedFooter struct {
	Text    string `json:"text"`
	IconURL string `json:"icon_url,omitempty"`
}

// DiscordWebhookMessage represents a Discord webhook message
type DiscordWebhookMessage struct {
	Content   string         `json:"content,omitempty"`
	Username  string         `json:"username,omitempty"`
	AvatarURL string         `json:"avatar_url,omitempty"`
	Embeds    []DiscordEmbed `json:"embeds,omitempty"`
}

// SendDiscordNotification sends game information to Discord via webhook
func SendDiscordNotification(webhookURL string, games []Game) error {
	if len(games) == 0 {
		return nil // No games to notify about
	}


	// Create webhook message
	message := DiscordWebhookMessage{
		Content:   "🎮 Free Games from Epic Games Store 🎮",
		Embeds:    []DiscordEmbed{},
	}

	// Add embeds for each game (Discord supports up to 10 embeds per message)
	for i, game := range games {
		if i >= 10 {
			break // Discord limit: maximum 10 embeds per message
		}
		message.Embeds = append(message.Embeds, createGameEmbed(game))
	}

	// Marshal the message to JSON
	payload, err := json.Marshal(message)
	if err != nil {
		return fmt.Errorf("error marshaling webhook message: %v", err)
	}

	// Create HTTP request
	req, err := http.NewRequest("POST", webhookURL, bytes.NewBuffer(payload))
	if err != nil {
		return fmt.Errorf("error creating webhook request: %v", err)
	}

	// Set headers
	req.Header.Set("Content-Type", "application/json")

	// Send the request
	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("error sending webhook request: %v", err)
	}
	defer resp.Body.Close()

	// Check response status
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("Discord webhook returned non-2xx status code: %d", resp.StatusCode)
	}

	return nil
}

// createGameEmbed creates a Discord embed for a game
func createGameEmbed(game Game) DiscordEmbed {
	// Set color based on game status
	color := 0x0078F2 // Epic Games blue color
	if game.Status == "free" {
		color = 0x2ECC71 // Green color for free games
	} else if game.Status == "coming soon" {
		color = 0xF1C40F // Yellow color for upcoming games
	}

	// Create embed
	embed := DiscordEmbed{
		Title:       game.Title,
		Description: game.Description,
		URL:         game.URL,
		Color:       color,
		Timestamp:   time.Now().Format(time.RFC3339),
		Fields:      []DiscordEmbedField{},
	}

	// Add publisher field if available
	if game.Publisher != "" {
		embed.Fields = append(embed.Fields, DiscordEmbedField{
			Name:   "Publisher",
			Value:  game.Publisher,
			Inline: true,
		})
	}

	// Add status field
	statusText := "Currently Free"
	if game.Status == "coming soon" {
		statusText = "Coming Soon"
	}
	embed.Fields = append(embed.Fields, DiscordEmbedField{
		Name:   "Status",
		Value:  statusText,
		Inline: true,
	})

	// Add dates fields if they're not unknown
	if game.StartDate != "Unknown" {
		embed.Fields = append(embed.Fields, DiscordEmbedField{
			Name:   "Available From",
			Value:  game.StartDate,
			Inline: false,
		})
	}
	if game.EndDate != "Unknown" {
		embed.Fields = append(embed.Fields, DiscordEmbedField{
			Name:   "Available Until",
			Value:  game.EndDate,
			Inline: false,
		})
	}

	// Add thumbnail if image URL is available
	if game.ImageURL != "" {
		embed.Thumbnail = &DiscordEmbedThumbnail{
			URL: game.ImageURL,
		}
	}

	// Add footer with date precision
	precisionText := ""
	switch game.DatePrecision {
	case "exact":
		precisionText = "Dates are exact"
	case "estimated":
		precisionText = "Dates are estimated"
	case "unknown":
		precisionText = "Dates are unknown"
	}
	
	embed.Footer = &DiscordEmbedFooter{
		Text: precisionText,
	}

	return embed
} 