package services

import (
	"encoding/json"
	"log"
	"sync"

	"server-content/internal/db/models"
)

// ─── Advert Hobby (advert_hobby) ─────────────────────────────────────

// AdvertHobby represents the advert_hobby setting value.
// Fields contain Ad document IDs (not full objects).
type AdvertHobby struct {
	Vdo        []string `json:"vdo"`
	Image      []string `json:"image"`
	Javascript []string `json:"javascript"`
}

// GetAdvertHobby reads advert_hobby from setting.json
func GetAdvertHobby() AdvertHobby {
	settings, err := ReadSettingFile()
	if err != nil {
		return AdvertHobby{}
	}
	raw, exists := settings["advert_hobby"]
	if !exists {
		return AdvertHobby{}
	}
	var result AdvertHobby
	if err := json.Unmarshal(raw, &result); err != nil {
		log.Printf("⚠️ Cannot parse advert_hobby: %v", err)
		return AdvertHobby{}
	}
	return result
}

// FindAdByID looks up an ad by ID from the in-memory ads cache.
func FindAdByID(adID string) *models.Ads {
	adCacheMu.RLock()
	defer adCacheMu.RUnlock()

	for _, ads := range adCache {
		for i := range ads {
			if ads[i].ID == adID {
				return &ads[i]
			}
		}
	}
	return nil
}

// ─── Advert Video (advert_vdo) ────────────────────────────────────────

// AdvertVdoItem represents a single video ad entry
type AdvertVdoItem struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	MP4Url      string `json:"mp4Url"`
	WebsiteUrl  string `json:"websiteUrl"`
	SkipSeconds int    `json:"skipSeconds"`
	IsActive    bool   `json:"isActive"`
}

// AdvertVdo represents the advert_vdo setting
type AdvertVdo struct {
	IsActive bool            `json:"isActive"` // master switch
	Items    []AdvertVdoItem `json:"items"`
}

// GetAdvertVdo reads advert_vdo settings from setting.json
// Format: array of AdvertVdoItem directly (not wrapped in object)
func GetAdvertVdo() []AdvertVdoItem {
	settings, err := ReadSettingFile()
	if err != nil {
		log.Printf("⚠️ Cannot read setting.json (advert_vdo): %v", err)
		return nil
	}
	raw, exists := settings["advert_vdo"]
	if !exists {
		return nil
	}
	var items []AdvertVdoItem
	if err := json.Unmarshal(raw, &items); err != nil {
		log.Printf("⚠️ Cannot parse advert_vdo: %v", err)
		return nil
	}
	return items
}

// ─── Advert Image (advert_image) ─────────────────────────────────────

// AdvertImage represents the advert_image setting
type AdvertImageSetting struct {
	IsActive   bool     `json:"isActive"`
	ImageUrl   string   `json:"imageUrl"`
	WebsiteUrl string   `json:"websiteUrl"`
	ShowOn     []string `json:"showOn"` // ready, end, pause
}

// GetAdvertImage reads advert_image settings from setting.json
func GetAdvertImage() AdvertImageSetting {
	defaults := AdvertImageSetting{}
	settings, err := ReadSettingFile()
	if err != nil {
		log.Printf("⚠️ Cannot read setting.json (advert_image): %v", err)
		return defaults
	}
	raw, exists := settings["advert_image"]
	if !exists {
		return defaults
	}
	var result AdvertImageSetting
	if err := json.Unmarshal(raw, &result); err != nil {
		log.Printf("⚠️ Cannot parse advert_image: %v", err)
		return defaults
	}
	return result
}

// ─── Advert Javascript (advert_javascript) ───────────────────────────

// AdvertJavascriptSetting represents the advert_javascript setting
// Format: plain string (raw script HTML)
type AdvertJavascriptSetting struct {
	Code string // the raw value from setting.json
}

// GetAdvertJavascript reads advert_javascript settings from setting.json
// Format: plain string directly
func GetAdvertJavascript() AdvertJavascriptSetting {
	settings, err := ReadSettingFile()
	if err != nil {
		log.Printf("⚠️ Cannot read setting.json (advert_javascript): %v", err)
		return AdvertJavascriptSetting{}
	}
	raw, exists := settings["advert_javascript"]
	if !exists {
		return AdvertJavascriptSetting{}
	}
	// Try as plain string first
	var code string
	if err := json.Unmarshal(raw, &code); err != nil {
		log.Printf("⚠️ Cannot parse advert_javascript: %v", err)
		return AdvertJavascriptSetting{}
	}
	return AdvertJavascriptSetting{Code: code}
}

// ─── Ads Cache (new system) ───────────────────────────────────────────

var (
	adCache   map[string][]models.Ads // spaceId → active ads
	adCacheMu sync.RWMutex
)

// LoadAds loads ads into the in-memory cache grouped by spaceId.
func LoadAds(ads []models.Ads) {
	cache := make(map[string][]models.Ads)
	for i := range ads {
		ad := ads[i]
		if ad.Status != "active" {
			continue
		}
		cache[ad.SpaceID] = append(cache[ad.SpaceID], ad)
	}

	adCacheMu.Lock()
	adCache = cache
	adCacheMu.Unlock()

	log.Printf("📋 Loaded %d active ads → ads cache", len(ads))
}

// FindAdsBySpaceID returns active ads for a given spaceId from the cache.
func FindAdsBySpaceID(spaceID string) []models.Ads {
	if spaceID == "" {
		return nil
	}

	adCacheMu.RLock()
	defer adCacheMu.RUnlock()

	return adCache[spaceID]
}

// ─── Resolved Ad Config ───────────────────────────────────────────────

// ResolvedAds is the final ad configuration passed to the player/vast
type ResolvedAds struct {
	// VAST video ads
	VastEnabled bool
	VdoItems    []AdvertVdoItem

	// Image overlay ads (multiple supported, JS picks randomly)
	AdvertImages []AdvertImageConfig

	// Javascript ad scripts (all injected)
	AdJavascripts []string
}

// ResolveAdsFromAds converts []models.Ads into ResolvedAds for paid plan.
func ResolveAdsFromAds(ads []models.Ads) ResolvedAds {
	result := ResolvedAds{}

	for _, ad := range ads {
		if ad.Content == nil {
			continue
		}
		switch ad.Type {
		case "video":
			if ad.Content.Mp4URL != nil && *ad.Content.Mp4URL != "" {
				result.VastEnabled = true
			}
		case "image":
			if ad.Content.ImageURL != nil && *ad.Content.ImageURL != "" {
				websiteUrl := ""
				if ad.Content.WebsiteURL != nil {
					websiteUrl = *ad.Content.WebsiteURL
				}
				result.AdvertImages = append(result.AdvertImages, AdvertImageConfig{
					ImageUrl:   *ad.Content.ImageURL,
					WebsiteUrl: websiteUrl,
					ShowOn:     ad.Content.ShowOn,
				})
			}
		case "script", "javascript":
			if ad.Content.Script != nil && *ad.Content.Script != "" {
				result.AdJavascripts = append(result.AdJavascripts, *ad.Content.Script)
			}
		}
	}

	return result
}

// ResolveAdsFromPlan selects ad config based on plan type.
//
//   - planType "hobby" or "" → ads from advert_hobby setting
//   - planType "pro"/"business"/"enterprise" → use ads from ads collection (by spaceId)
func ResolveAdsFromPlan(planType string, spaceID string) ResolvedAds {
	if planType != "" && planType != models.PlanTypeHobby && spaceID != "" {
		ads := FindAdsBySpaceID(spaceID)
		if len(ads) > 0 {
			return ResolveAdsFromAds(ads)
		}
		// No ads configured for this space → no ads
		return ResolvedAds{}
	}

	// Hobby (or no plan): read Ad IDs from advert_hobby, lookup from ads cache
	hobby := GetAdvertHobby()
	result := ResolvedAds{}

	// Resolve video ad IDs → enable VAST if any active
	for _, adID := range hobby.Vdo {
		ad := FindAdByID(adID)
		if ad != nil && ad.Content != nil && ad.Content.Mp4URL != nil && *ad.Content.Mp4URL != "" {
			result.VastEnabled = true
			break
		}
	}

	// Resolve image ad IDs
	for _, adID := range hobby.Image {
		ad := FindAdByID(adID)
		if ad != nil && ad.Content != nil && ad.Content.ImageURL != nil && *ad.Content.ImageURL != "" {
			websiteUrl := ""
			if ad.Content.WebsiteURL != nil {
				websiteUrl = *ad.Content.WebsiteURL
			}
			result.AdvertImages = append(result.AdvertImages, AdvertImageConfig{
				ImageUrl:   *ad.Content.ImageURL,
				WebsiteUrl: websiteUrl,
				ShowOn:     ad.Content.ShowOn,
			})
		}
	}

	// Resolve javascript ad IDs
	for _, adID := range hobby.Javascript {
		ad := FindAdByID(adID)
		if ad != nil && ad.Content != nil && ad.Content.Script != nil && *ad.Content.Script != "" {
			result.AdJavascripts = append(result.AdJavascripts, *ad.Content.Script)
		}
	}

	return result
}

// ─── Legacy helpers ───────────────────────────────────────────────────

// IsAdvertVideoEnabled returns whether global video ads have any active items
func IsAdvertVideoEnabled() bool {
	items := GetAdvertVdo()
	for _, item := range items {
		if item.IsActive {
			return true
		}
	}
	return false
}
