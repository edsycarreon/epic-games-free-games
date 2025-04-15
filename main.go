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
	"github.com/robfig/cron/v3"
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

type APIResponse struct {
	Success bool   `json:"success"`
	Message string `json:"message,omitempty"`
	Count   int    `json:"count"`
	Data    []Game `json:"data"`
}

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

func getEnvString(key, defaultValue string) string {
	value := os.Getenv(key)
	if value == "" {
		return defaultValue
	}
	return value
}

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

func getEnvBool(key string, defaultValue bool) bool {
	value := os.Getenv(key)
	if value == "" {
		return defaultValue
	}
	boolValue, err := strconv.ParseBool(value)
	if err != nil {
		log.Printf("Warning: Environment variable %s is not a valid boolean, using default: %v\n", key, defaultValue)
		return defaultValue
	}
	return boolValue
}

func main() {
	// Load .env file
	err := godotenv.Load()
	if err != nil {
		log.Println("Warning: Error loading .env file:", err)
	}
	
	port := flag.Int("port", getEnvInt("PORT", 8080), "Port for the API server to listen on")
	
	discordWebhook := flag.String("discord-webhook", os.Getenv("DISCORD_WEBHOOK_URL"), "Discord webhook URL for notifications")
	
	countryCode := flag.String("country", getEnvString("COUNTRY_CODE", "PH"), "Country code for Epic Games Store")
	locale := flag.String("locale", getEnvString("LOCALE", "en-PH"), "Locale for Epic Games Store")
	timezone := flag.String("timezone", getEnvString("TIMEZONE", "Asia/Manila"), "Timezone for date/time formatting")
	
	enableCron := flag.Bool("enable-cron", getEnvBool("ENABLE_CRON", false), "Enable built-in cron job to check for free games")
	cronSchedule := flag.String("cron-schedule", getEnvString("CRON_SCHEDULE", "0 0 0 * * *"), "Cron schedule expression for checking free games")
	
	flag.Parse()

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

	// Set up cron job if enabled
	if *enableCron {
		setupCronJob(*cronSchedule, *countryCode, *locale, *timezone, *discordWebhook)
	}

	fmt.Printf("Epic Games API server listening on port %d...\n", *port)
	log.Fatal(http.ListenAndServe(fmt.Sprintf(":%d", *port), nil))
}

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
			sendNotification = notifyBool && webhookURL != ""
		}
	} else {
		sendNotification = webhookURL != ""
	}

	games, err := fetchFreeGames(countryCode, locale, includeUpcoming, timezone)
	
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Access-Control-Allow-Origin", "*")

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

	response := APIResponse{
		Success: true,
		Count:   len(games),
		Data:    games,
	}
	
	jsonData, _ := json.MarshalIndent(response, "", "  ")
	w.Write(jsonData)
}

func fetchFreeGames(countryCode, locale string, includeUpcoming bool, timezone string) ([]Game, error) {
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

	req, err := http.NewRequest("POST", "https://graphql.epicgames.com/graphql", bytes.NewBuffer(requestBody))
	if err != nil {
		return nil, fmt.Errorf("error creating request: %v", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/91.0.4472.124 Safari/537.36")

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("error sending request: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("bad status: %d, response: %s", resp.StatusCode, string(bodyBytes))
	}

	var graphQLResp GraphQLResponse
	if err := json.NewDecoder(resp.Body).Decode(&graphQLResp); err != nil {
		return nil, fmt.Errorf("error decoding response: %v", err)
	}

	var games []Game
	for _, element := range graphQLResp.Data.Catalog.SearchStore.Elements {
		game := Game{
			Title:       element.Title,
			Description: element.Description,
			Publisher:   element.Seller.Name,
		}

		for _, img := range element.KeyImages {
			if img.Type == "Thumbnail" || img.Type == "DieselGameBox" {
				game.ImageURL = img.URL
				break
			}
		}

		pageSlug := ""
		if len(element.OfferMappings) > 0 {
			for _, mapping := range element.OfferMappings {
				if mapping.PageSlug != "" {
					pageSlug = mapping.PageSlug
					break
				}
			}
		}
		
		if pageSlug == "" && len(element.CatalogNs.Mappings) > 0 {
			for _, mapping := range element.CatalogNs.Mappings {
				if mapping.PageSlug != "" {
					pageSlug = mapping.PageSlug
					break
				}
			}
		}

		game.URL = fmt.Sprintf("https://store.epicgames.com/en-US/p/%s", pageSlug)

		isCurrentlyFree := false
		hasUpcomingFree := false
		
		formatDate := func(dateStr string) string {
			t, err := time.Parse(time.RFC3339, dateStr)
			if err != nil {
				return dateStr
			}
			
			location, err := time.LoadLocation(timezone)
			if err != nil {
				if strings.HasPrefix(timezone, "UTC") || strings.HasPrefix(timezone, "GMT") {
					offsetStr := strings.TrimPrefix(strings.TrimPrefix(timezone, "UTC"), "GMT")
					if offsetStr == "" {
						location = time.UTC
					} else {
						offsetHours := 0
						if _, err := fmt.Sscanf(offsetStr, "%d", &offsetHours); err == nil {
							location = time.FixedZone(timezone, offsetHours*60*60)
						} else {
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
						if promo.DiscountSetting.DiscountPercentage == 100 {
							isCurrentlyFree = true
							game.Status = "free"
							game.StartDate = formatDate(promo.StartDate)
							game.EndDate = formatDate(promo.EndDate)
							game.DatePrecision = "exact"
						}
					}
				}
			}
		}

		if !isCurrentlyFree && includeUpcoming && len(element.Promotions.UpcomingPromotionalOffers) > 0 {
			for _, offer := range element.Promotions.UpcomingPromotionalOffers {
				if len(offer.PromotionalOffers) > 0 {
					for _, promo := range offer.PromotionalOffers {
						if promo.DiscountSetting.DiscountPercentage == 100 {
							hasUpcomingFree = true
							game.Status = "coming soon"
							game.StartDate = formatDate(promo.StartDate)
							game.EndDate = formatDate(promo.EndDate)
							game.DatePrecision = "exact"
						}
					}
				}
			}
		}

		if !isCurrentlyFree && !hasUpcomingFree {
			price := element.Price.TotalPrice.FmtPrice.DiscountPrice
			if price == "$0.00" || price == "0" || price == "" || strings.Contains(strings.ToLower(price), "free") {
				game.Status = "free"
				
				location, err := time.LoadLocation(timezone)
				if err != nil {
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

		if !includeUpcoming && game.Status == "coming soon" {
			continue
		}
		
		if game.Status == "free" && (game.StartDate == "" && game.EndDate == "" || game.DatePrecision == "estimated") {
			
			if len(element.Promotions.PromotionalOffers) > 0 {
				for _, offer := range element.Promotions.PromotionalOffers {
					if len(offer.PromotionalOffers) > 0 {
						promo := offer.PromotionalOffers[0]
						
						if promo.StartDate != "" && promo.EndDate != "" {
							game.StartDate = formatDate(promo.StartDate)
							game.EndDate = formatDate(promo.EndDate)
							game.DatePrecision = "exact"
							break
						}
					}
				}
			}
			
			if game.DatePrecision == "estimated" {
				log.Printf("Game with estimated dates: %s (Status: %s)", game.Title, game.Status)
			}
		}

		if game.StartDate == "" && game.EndDate == "" {
			game.StartDate = "Unknown"
			game.EndDate = "Unknown"
			game.DatePrecision = "unknown"
		}

		games = append(games, game)
	}

	return games, nil
}

func setupCronJob(schedule, countryCode, locale, timezone, webhookURL string) {
	if webhookURL == "" {
		log.Println("Warning: Discord webhook URL not configured. Cron job will run but no notifications will be sent.")
	}

	c := cron.New(cron.WithSeconds())
	
	log.Printf("Setting up cron job with schedule: %s", schedule)
	
	_, err := c.AddFunc(schedule, func() {
		log.Println("Running scheduled free games check...")
		
		games, err := fetchFreeGames(countryCode, locale, true, timezone)
		if err != nil {
			log.Printf("Error fetching free games: %v", err)
			return
		}
			
		log.Printf("Found %d free game(s)", len(games))
		
		// Send notification to Discord if webhook URL is configured
		if webhookURL != "" {
			err = SendDiscordNotification(webhookURL, games)
			if err != nil {
				log.Printf("Error sending Discord notification: %v", err)
			} else {
					log.Printf("Discord notification sent for %d games", len(games))
			}
		}
	})
	
	if err != nil {
		log.Printf("Error setting up cron job: %v", err)
		return
	}
	
	c.Start()
	log.Println("Cron scheduler started")
}
