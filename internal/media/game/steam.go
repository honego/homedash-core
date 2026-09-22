package game

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/honeok/homedash-core/internal/config"
	"golang.org/x/sync/errgroup"
)

const (
	playerSummariesURL     = "https://api.steampowered.com/ISteamUser/GetPlayerSummaries/v2/"         // Steam 玩家资料 头像及在线状态
	steamLevelURL          = "https://api.steampowered.com/IPlayerService/GetSteamLevel/v1/"          // Steam 玩家等级
	recentlyPlayedGamesURL = "https://api.steampowered.com/IPlayerService/GetRecentlyPlayedGames/v1/" // Steam 最近游玩及游玩时长
	storeItemsURL          = "https://api.steampowered.com/IStoreBrowseService/GetItems/v1/"          // Steam 游戏商店资源及封面
	requestTimeout         = 10 * time.Second
	maxResponseSize        = 4 << 20
)

// Steam 接口
type Steam struct {
	client  *http.Client
	apiKey  string
	steamID string
}

// Steam 数据
type Data struct {
	Profile     json.RawMessage `json:"profile"`
	Level       json.RawMessage `json:"level"`
	RecentGames json.RawMessage `json:"recent_games"`
	StoreItems  json.RawMessage `json:"store_items"`
}

// 创建 Steam 接口
func NewSteam(cfg config.SteamConfig) *Steam {
	return &Steam{
		client:  &http.Client{Timeout: requestTimeout},
		apiKey:  strings.TrimSpace(cfg.APIKey),
		steamID: strings.TrimSpace(cfg.SteamID),
	}
}

// 获取 Steam 数据
func (s *Steam) Get(ctx context.Context) (Data, error) {
	group, ctx := errgroup.WithContext(ctx)
	var profile, level, recentGames, storeItems json.RawMessage

	group.Go(func() error {
		var err error
		profile, err = s.get(ctx, "profile", playerSummariesURL, url.Values{
			"key":      []string{s.apiKey},
			"steamids": []string{s.steamID},
		})
		return err
	})
	group.Go(func() error {
		var err error
		level, err = s.get(ctx, "level", steamLevelURL, url.Values{
			"key":     []string{s.apiKey},
			"steamid": []string{s.steamID},
		})
		return err
	})
	group.Go(func() error {
		var err error
		recentGames, err = s.get(ctx, "recent games", recentlyPlayedGamesURL, url.Values{
			"key":     []string{s.apiKey},
			"steamid": []string{s.steamID},
		})
		if err != nil {
			return err
		}
		storeItems, err = s.getStoreItems(ctx, recentGames)
		return err
	})

	if err := group.Wait(); err != nil {
		return Data{}, err
	}
	return Data{
		Profile:     profile,
		Level:       level,
		RecentGames: recentGames,
		StoreItems:  storeItems,
	}, nil
}

// 获取商店数据
func (s *Steam) getStoreItems(ctx context.Context, recentGames json.RawMessage) (json.RawMessage, error) {
	var recent struct {
		Response struct {
			Games []struct {
				AppID uint32 `json:"appid"`
			} `json:"games"`
		} `json:"response"`
	}
	if err := json.Unmarshal(recentGames, &recent); err != nil {
		return nil, fmt.Errorf("failed to parse recent games response: %w", err)
	}
	if len(recent.Response.Games) == 0 {
		return json.RawMessage(`{"response":{"store_items":[]}}`), nil
	}

	input, err := json.Marshal(map[string]any{
		"ids":          recent.Response.Games,
		"context":      map[string]string{"country_code": "US"},
		"data_request": map[string]bool{"include_assets": true},
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create store items request: %w", err)
	}
	return s.get(ctx, "store items", storeItemsURL, url.Values{
		"key":        []string{s.apiKey},
		"input_json": []string{string(input)},
	})
}

// 请求 Steam 接口
func (s *Steam) get(ctx context.Context, name string, endpoint string, query url.Values) (json.RawMessage, error) {
	requestURL, err := url.Parse(endpoint)
	if err != nil {
		return nil, fmt.Errorf("%s request URL is invalid: %w", name, err)
	}

	request, err := http.NewRequestWithContext(ctx, http.MethodGet, requestURL.String(), nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create %s request: %w", name, err)
	}
	request.URL.RawQuery = query.Encode()
	request.Header.Set("Accept", "application/json")

	response, err := s.client.Do(request)
	if err != nil {
		if requestError, ok := err.(*url.Error); ok {
			err = requestError.Err
		}
		return nil, fmt.Errorf("%s request failed: %w", name, err)
	}
	defer response.Body.Close()

	body, err := io.ReadAll(io.LimitReader(response.Body, maxResponseSize+1))
	if err != nil {
		return nil, fmt.Errorf("failed to read %s response: %w", name, err)
	}
	if len(body) > maxResponseSize {
		return nil, fmt.Errorf("%s response is too large", name)
	}
	if response.StatusCode < http.StatusOK || response.StatusCode >= http.StatusMultipleChoices {
		return nil, fmt.Errorf("%s returned HTTP status %d", name, response.StatusCode)
	}
	if !json.Valid(body) {
		return nil, fmt.Errorf("%s returned invalid JSON", name)
	}

	return json.RawMessage(body), nil
}
