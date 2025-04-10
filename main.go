package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/joho/godotenv"
)

// Game represents a free game from Epic Games Store
type Game struct {
	Title         string `json:"title"`
	Description   string `json:"description,omitempty"`
	ImageURL      string `json:"image_url,omitempty"`
	URL           string `json:"url,omitempty"`
	Status        string `json:"status"` // "free" or "coming soon"
	StartDate     string `json:"start_date"`
	EndDate       string `json:"end_date"`
	DatePrecision string `json:"date_precision"` // "exact", "estimated", or "unknown"
	Publisher     string `json:"publisher,omitempty"`
}

// API response structure
type APIResponse struct {
	Success bool   `json:"success"`
	Message string `json:"message,omitempty"`
	Count   int    `json:"count"`
	Data    []Game `json:"data"`
}

// GraphQL query for free games
const freeGamesQuery = `
query searchStoreQuery(
  $category: String,
  $count: Int,
  $country: String!,
  $locale: String,
  $freeGame: Boolean,
  $onSale: Boolean,
  $withPrice: Boolean = true
) {
  Catalog {
    searchStore(
      category: $category
      count: $count
      country: $country
      freeGame: $freeGame
      onSale: $onSale
      locale: $locale
    ) {
      elements {
        title
        description
        seller {
          name
        }
        keyImages {
          type
          url
        }
        productSlug
        urlSlug
        url
        offerMappings {
          pageSlug
          pageType
        }
        catalogNs {
          mappings(pageType: "productHome") {
            pageSlug
            pageType
          }
        }
        linkedOffer {
          effectiveDate
          customAttributes{
            key
            value
          }
        }
        categories {
          path
        }
        namespace
        id
        price(country: $country) @include(if: $withPrice) {
          totalPrice {
            fmtPrice(locale: $locale) {
              discountPrice
              originalPrice
            }
          }
        }
        promotions {
          promotionalOffers {
            promotionalOffers {
              startDate
              endDate
              discountSetting {
                discountType
                discountPercentage
              }
            }
          }
          upcomingPromotionalOffers {
            promotionalOffers {
              startDate
              endDate
              discountSetting {
                discountType
                discountPercentage
              }
            }
          }
        }
      }
    }
  }
}
`

type GraphQLRequest struct {
	Query     string                 `json:"query"`
	Variables map[string]interface{} `json:"variables"`
}

type GraphQLResponse struct {
	Data struct {
		Catalog struct {
			SearchStore struct {
				Elements []struct {
					Title       string `json:"title"`
					Description string `json:"description"`
					Seller      struct {
						Name string `json:"name"`
					} `json:"seller"`
					KeyImages []struct {
						Type string `json:"type"`
						URL  string `json:"url"`
					} `json:"keyImages"`
					ProductSlug string `json:"productSlug"`
					URL         string `json:"url"`
					UrlSlug     string `json:"urlSlug"`
					OfferMappings []struct {
						PageSlug string `json:"pageSlug"`
						PageType string `json:"pageType"`
					} `json:"offerMappings"`
					CatalogNs struct {
						Mappings []struct {
							PageSlug string `json:"pageSlug"`
							PageType string `json:"pageType"`
						} `json:"mappings"`
					} `json:"catalogNs"`
					LinkedOffer struct {
						EffectiveDate string `json:"effectiveDate"`
						CustomAttributes []struct {
							Key   string `json:"key"`
							Value string `json:"value"`
						} `json:"customAttributes"`
					} `json:"linkedOffer"`
					Categories []struct {
						Path string `json:"path"`
					} `json:"categories"`
					Namespace string `json:"namespace"`
					ID        string `json:"id"`
					Price       struct {
						TotalPrice struct {
							FmtPrice struct {
								OriginalPrice string `json:"originalPrice"`
								DiscountPrice string `json:"discountPrice"`
							} `json:"fmtPrice"`
						} `json:"totalPrice"`
					} `json:"price"`
					Promotions struct {
						PromotionalOffers []struct {
							PromotionalOffers []struct {
								StartDate       string `json:"startDate"`
								EndDate         string `json:"endDate"`
								DiscountSetting struct {
									DiscountType       string `json:"discountType"`
									DiscountPercentage int    `json:"discountPercentage"`
								} `json:"discountSetting"`
							} `json:"promotionalOffers"`
						} `json:"promotionalOffers"`
						UpcomingPromotionalOffers []struct {
							PromotionalOffers []struct {
								StartDate       string `json:"startDate"`
								EndDate         string `json:"endDate"`
								DiscountSetting struct {
									DiscountType       string `json:"discountType"`
									DiscountPercentage int    `json:"discountPercentage"`
								} `json:"discountSetting"`
							} `json:"promotionalOffers"`
						} `json:"upcomingPromotionalOffers"`
					} `json:"promotions"`
				} `json:"elements"`
			} `json:"searchStore"`
		} `json:"Catalog"`
	} `json:"data"`
}

// getEnvString returns the value of the environment variable or the default value if not set
func getEnvString(key, defaultValue string) string {
	value := os.Getenv(key)
	if value == "" {
		return defaultValue
	}
	return value
}

// getEnvInt returns the integer value of the environment variable or the default value if not set
func getEnvInt(key string, defaultValue int) int {
	value := os.Getenv(key)
	if value == "" {
		return defaultValue
	}
	intValue, err := strconv.Atoi(value)
	if err != nil {
		log.Printf("Warning: Environment variable %s is not a valid integer, using default: %d\n", key, defaultValue)
		return defaultValue
	}
	return intValue
}

func main() {
	// Load .env file
	err := godotenv.Load()
	if err != nil {
		log.Println("Warning: Error loading .env file:", err)
	}
	
	// Define command-line flags with environment variable defaults
	port := flag.Int("port", getEnvInt("PORT", 8080), "Port for the API server to listen on")
	
	// Discord webhook configuration
	discordWebhook := flag.String("discord-webhook", os.Getenv("DISCORD_WEBHOOK_URL"), "Discord webhook URL for notifications")
	
	
	// Region and locale configuration
	countryCode := flag.String("country", getEnvString("COUNTRY_CODE", "PH"), "Country code for Epic Games Store")
	locale := flag.String("locale", getEnvString("LOCALE", "en-PH"), "Locale for Epic Games Store")
	timezone := flag.String("timezone", getEnvString("TIMEZONE", "Asia/Manila"), "Timezone for date/time formatting")
	
	flag.Parse()

	// Set up API routes
	http.HandleFunc("/api/free-games", func(w http.ResponseWriter, r *http.Request) {
		freeGamesHandler(w, r, *countryCode, *locale, *timezone, *discordWebhook)
	})
	http.HandleFunc("/", indexHandler)
	
	// Set up Discord webhook notification route (for manual triggering)
	http.HandleFunc("/notify", func(w http.ResponseWriter, r *http.Request) {
		if *discordWebhook == "" {
			http.Error(w, "Discord webhook URL not configured", http.StatusInternalServerError)
			return
		}
		
		// Get free games
		games, err := fetchFreeGames(*countryCode, *locale, true, *timezone)
		if err != nil {
			http.Error(w, fmt.Sprintf("Error fetching games: %v", err), http.StatusInternalServerError)
			return
		}
		
		// Send notification to Discord
		err = SendDiscordNotification(*discordWebhook, games)
		if err != nil {
			http.Error(w, fmt.Sprintf("Error sending Discord notification: %v", err), http.StatusInternalServerError)
			return
		}
		
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"success": true,
			"message": fmt.Sprintf("Notification sent for %d games", len(games)),
		})
	})

	// Start the server
	fmt.Printf("Epic Games API server listening on port %d...\n", *port)
	log.Fatal(http.ListenAndServe(fmt.Sprintf(":%d", *port), nil))
}

// indexHandler serves a simple HTML page with information about the API
func indexHandler(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}

	html := `
	<!DOCTYPE html>
	<html lang="en">
	<head>
		<meta charset="UTF-8">
		<meta name="viewport" content="width=device-width, initial-scale=1.0">
		<title>Epic Games Free Games API</title>
		<style>
			body {
				font-family: Arial, sans-serif;
				line-height: 1.6;
				margin: 0 auto;
				max-width: 800px;
				padding: 20px;
			}
			h1 {
				color: #0078f2;
			}
			pre {
				background-color: #f5f5f5;
				border-radius: 5px;
				padding: 15px;
				overflow-x: auto;
			}
			code {
				font-family: monospace;
			}
		</style>
	</head>
	<body>
		<h1>Epic Games Free Games API</h1>
		<p>Use this API to get information about free games available on the Epic Games Store.</p>
		
		<h2>Endpoints</h2>
		<h3>GET /api/free-games</h3>
		<p>Returns all free games currently available and upcoming free games.</p>
		
		<h4>Query Parameters</h4>
		<ul>
			<li><code>upcoming</code> - Include upcoming free games (true/false, default: true)</li>
			<li><code>country</code> - Country code for the store (default: PH)</li>
			<li><code>locale</code> - Locale for text formatting (default: en-PH)</li>
			<li><code>timezone</code> - Timezone for dates (default: Asia/Manila). Use standard IANA timezone names like "America/New_York", "Europe/London", or UTC offsets like "UTC+1"</li>
		</ul>
		
		<h4>Example Request</h4>
		<pre><code>GET /api/free-games?upcoming=false&timezone=America/New_York</code></pre>
		
		<h4>Example Response</h4>
		<pre><code>{
  "success": true,
  "count": 1,
  "data": [
    {
      "title": "Game Title",
      "description": "Game description",
      "image_url": "https://example.com/image.jpg",
      "url": "https://store.epicgames.com/en-US/p/game-slug",
      "status": "free",
      "start_date": "2025-04-04 15:00:00 PHT",
      "end_date": "2025-04-11 15:00:00 PHT",
      "date_precision": "exact",
      "publisher": "Publisher Name"
    }
  ]
}</code></pre>

        <h4>Date Fields</h4>
        <p>The <code>start_date</code> and <code>end_date</code> fields show when a game is or will be available for free. Times are displayed in the requested timezone or Philippine Time (UTC+8) by default.</p>
        
        <h4>Date Precision Field</h4>
        <p>The <code>date_precision</code> field indicates how accurate the start and end dates are:</p>
        <ul>
            <li><strong>exact</strong>: Dates are directly from Epic Games' API</li>
            <li><strong>estimated</strong>: Dates are estimated based on typical free game periods</li>
            <li><strong>unknown</strong>: Unable to determine accurate dates</li>
        </ul>
	</body>
	</html>
	`

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	fmt.Fprint(w, html)
}

// freeGamesHandler handles requests to the /api/free-games endpoint
func freeGamesHandler(w http.ResponseWriter, r *http.Request, countryCode, locale, timezone, 
					  webhookURL string) {
	// Set default values
	includeUpcoming := true
	sendNotification := false // Flag to determine if we should send Discord notification

	// Get query parameters
	if upcoming := r.URL.Query().Get("upcoming"); upcoming != "" {
		if upcomingBool, err := strconv.ParseBool(upcoming); err == nil {
			includeUpcoming = upcomingBool
		}
	}
	
	// Check if this request should trigger a notification
	if notify := r.URL.Query().Get("notify"); notify != "" {
		if notifyBool, err := strconv.ParseBool(notify); err == nil {
			// If webhook URL is available, Discord notifications are enabled
			sendNotification = notifyBool && webhookURL != ""
		}
	} else {
		// By default, send notification if webhook URL is available
		sendNotification = webhookURL != ""
	}

	// Get free games
	games, err := fetchFreeGames(countryCode, locale, includeUpcoming, timezone)
	
	// Prepare response
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Access-Control-Allow-Origin", "*")

	// Handle errors
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		response := APIResponse{
			Success: false,
			Message: fmt.Sprintf("Error fetching games: %v", err),
			Count:   0,
			Data:    nil,
		}
		json.NewEncoder(w).Encode(response)
		return
	}

	// Send Discord notification if requested and enabled
	if sendNotification {
		if webhookURL != "" {
	
			err := SendDiscordNotification(webhookURL, games)
			if err != nil {
				log.Printf("Error sending Discord notification: %v", err)
			} else {
				log.Printf("Discord notification sent for %d games", len(games))
			}
		} else {
			log.Printf("Discord webhook URL not configured")
		}
	}

	// Return successful response
	response := APIResponse{
		Success: true,
		Count:   len(games),
		Data:    games,
	}
	
	// Return pretty JSON for better readability
	jsonData, _ := json.MarshalIndent(response, "", "  ")
	w.Write(jsonData)
}

// fetchFreeGames gets free games from Epic Games Store
func fetchFreeGames(countryCode, locale string, includeUpcoming bool, timezone string) ([]Game, error) {
	// Prepare GraphQL request
	variables := map[string]interface{}{
		"category": "games/edition/base|bundles/games|editors",
		"count":    100,
		"country":  countryCode,
		"locale":   locale,
		"freeGame": true,
		"onSale":   true,
	}

	requestBody, err := json.Marshal(GraphQLRequest{
		Query:     freeGamesQuery,
		Variables: variables,
	})
	if err != nil {
		return nil, fmt.Errorf("error marshaling request: %v", err)
	}

	// Create HTTP request
	req, err := http.NewRequest("POST", "https://graphql.epicgames.com/graphql", bytes.NewBuffer(requestBody))
	if err != nil {
		return nil, fmt.Errorf("error creating request: %v", err)
	}

	// Set headers
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/91.0.4472.124 Safari/537.36")

	// Send request
	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("error sending request: %v", err)
	}
	defer resp.Body.Close()

	// Check response status
	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("bad status: %d, response: %s", resp.StatusCode, string(bodyBytes))
	}

	// Parse response
	var graphQLResp GraphQLResponse
	if err := json.NewDecoder(resp.Body).Decode(&graphQLResp); err != nil {
		return nil, fmt.Errorf("error decoding response: %v", err)
	}

	// Convert to Game structs
	var games []Game
	for _, element := range graphQLResp.Data.Catalog.SearchStore.Elements {
		game := Game{
			Title:       element.Title,
			Description: element.Description,
			Publisher:   element.Seller.Name,
		}

		// Get the thumbnail image
		for _, img := range element.KeyImages {
			if img.Type == "Thumbnail" || img.Type == "DieselGameBox" {
				game.ImageURL = img.URL
				break
			}
		}

		// Construct URL with all available information
		// Remove debugging logs but keep the logic
			
		// Check for page slug in offer mappings
		pageSlug := ""
		if len(element.OfferMappings) > 0 {
			for _, mapping := range element.OfferMappings {
				if mapping.PageSlug != "" {
					pageSlug = mapping.PageSlug
					break
				}
			}
		}
		
		// Check catalog mappings as another source
		if pageSlug == "" && len(element.CatalogNs.Mappings) > 0 {
			for _, mapping := range element.CatalogNs.Mappings {
				if mapping.PageSlug != "" {
					pageSlug = mapping.PageSlug
					break
				}
			}
		}

		game.URL = fmt.Sprintf("https://store.epicgames.com/en-US/p/%s", pageSlug)

		// Check if it's currently free
		isCurrentlyFree := false
		hasUpcomingFree := false
		
		// Format dates to be more readable and convert to specified timezone
		formatDate := func(dateStr string) string {
			// Parse the date string (usually in RFC3339 format)
			t, err := time.Parse(time.RFC3339, dateStr)
			if err != nil {
				// If we can't parse, return the original string
				return dateStr
			}
			
			// Try to load the specified timezone
			location, err := time.LoadLocation(timezone)
			if err != nil {
				// If the timezone is invalid, try to parse it as a UTC offset
				if strings.HasPrefix(timezone, "UTC") || strings.HasPrefix(timezone, "GMT") {
					// Try to extract offset
					offsetStr := strings.TrimPrefix(strings.TrimPrefix(timezone, "UTC"), "GMT")
					if offsetStr == "" {
						// Just UTC+0
						location = time.UTC
					} else {
						// Parse hours offset
						offsetHours := 0
						if _, err := fmt.Sscanf(offsetStr, "%d", &offsetHours); err == nil {
							location = time.FixedZone(timezone, offsetHours*60*60)
						} else {
							// Default to Philippine timezone if parse fails
							location = time.FixedZone("UTC+8", 8*60*60)
						}
					}
				} else {
					// Default to Philippine timezone if loading fails
					location = time.FixedZone("UTC+8", 8*60*60)
				}
			}
			
			// Convert the time to the specified timezone
			tzTime := t.In(location)
			
			// Format in a readable format with timezone indicator
			return tzTime.Format("2006-01-02 15:04:05 MST")
		}

		// Find promotion dates (current promotions have priority)
		if len(element.Promotions.PromotionalOffers) > 0 {
			for _, offer := range element.Promotions.PromotionalOffers {
				if len(offer.PromotionalOffers) > 0 {
					for _, promo := range offer.PromotionalOffers {
						// Check if it's a 100% discount
						if promo.DiscountSetting.DiscountPercentage == 100 {
							isCurrentlyFree = true
							game.Status = "free"
							// Store original dates
							game.StartDate = formatDate(promo.StartDate)
							game.EndDate = formatDate(promo.EndDate)
							game.DatePrecision = "exact"
						}
					}
				}
			}
		}

		// Check upcoming promotions if we include upcoming free games
		if !isCurrentlyFree && includeUpcoming && len(element.Promotions.UpcomingPromotionalOffers) > 0 {
			for _, offer := range element.Promotions.UpcomingPromotionalOffers {
				if len(offer.PromotionalOffers) > 0 {
					for _, promo := range offer.PromotionalOffers {
						// Check if it will be a 100% discount
						if promo.DiscountSetting.DiscountPercentage == 100 {
							hasUpcomingFree = true
							game.Status = "coming soon"
							// Store original dates
							game.StartDate = formatDate(promo.StartDate)
							game.EndDate = formatDate(promo.EndDate)
							game.DatePrecision = "exact"
						}
					}
				}
			}
		}

		// Check price if it's 0, it's free
		if !isCurrentlyFree && !hasUpcomingFree {
			price := element.Price.TotalPrice.FmtPrice.DiscountPrice
			if price == "$0.00" || price == "0" || price == "" || strings.Contains(strings.ToLower(price), "free") {
				game.Status = "free"
				
				// Try to load the specified timezone
				location, err := time.LoadLocation(timezone)
				if err != nil {
					// Default to Philippine timezone if loading fails
					location = time.FixedZone("UTC+8", 8*60*60)
				}
				
				// Get current time in specified timezone
				now := time.Now().In(location)
				// Set approximate end date to a week from now if we can't find real dates
				endDate := now.AddDate(0, 0, 7)
				
				game.StartDate = now.Format("2006-01-02 15:04:05 MST")
				game.EndDate = endDate.Format("2006-01-02 15:04:05 MST")
				game.DatePrecision = "estimated"
			} else {
				// Skip non-free games
				continue
			}
		}

		// Skip upcoming games if not requested
		if !includeUpcoming && game.Status == "coming soon" {
			continue
		}

		// Ensure we don't have empty dates for free games by checking again
		if game.Status == "free" && (game.StartDate == "" && game.EndDate == "" || game.DatePrecision == "estimated") {
			
			// Try to extract dates from all promotions as a fallback - look more deeply
			if len(element.Promotions.PromotionalOffers) > 0 {
				for _, offer := range element.Promotions.PromotionalOffers {
					if len(offer.PromotionalOffers) > 0 {
						// Just use the first promotion's dates if we haven't found a 100% discount
						promo := offer.PromotionalOffers[0]
						
						// Only replace estimated dates if we have actual dates
						if promo.StartDate != "" && promo.EndDate != "" {
							game.StartDate = formatDate(promo.StartDate)
							game.EndDate = formatDate(promo.EndDate)
							game.DatePrecision = "exact"
							break
						}
					}
				}
			}
			
			// Debug: log the game that has estimated dates
			if game.DatePrecision == "estimated" {
				log.Printf("Game with estimated dates: %s (Status: %s)", game.Title, game.Status)
			}
		}

		// If we still don't have dates, mark it as unknown
		if game.StartDate == "" && game.EndDate == "" {
			game.StartDate = "Unknown"
			game.EndDate = "Unknown"
			game.DatePrecision = "unknown"
		}

		games = append(games, game)
	}

	return games, nil
}

// checkURL verifies if a URL is valid by making a HEAD request
func checkURL(url string) bool {
	client := &http.Client{
		Timeout: 5 * time.Second,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			// Allow up to 10 redirects
			if len(via) >= 10 {
				return fmt.Errorf("too many redirects")
			}
			return nil
		},
	}
	
	req, err := http.NewRequest("HEAD", url, nil)
	if err != nil {
		return false
	}
	
	// Set user agent to avoid being blocked
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/91.0.4472.124 Safari/537.36")
	
	resp, err := client.Do(req)
	if err != nil {
		return false
	}
	defer resp.Body.Close()
	
	// Status codes 200-399 are considered valid
	return resp.StatusCode >= 200 && resp.StatusCode < 400
}
