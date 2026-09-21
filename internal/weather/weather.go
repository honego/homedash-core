package weather

import (
	"context"
	"crypto"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	neturl "net/url"
	"os"
	"strconv"
	"strings"
	"time"

	jwt "github.com/golang-jwt/jwt/v5"
	"github.com/honeok/homepage-core/internal/config"
)

const (
	requestTimeout  = 10 * time.Second
	jwtLifetime     = 15 * time.Minute
	jwtClockSkew    = 30 * time.Second
	maxResponseSize = 4 << 20
)

// 天气接口
type Handler struct {
	client       *http.Client
	baseURL      string
	developerID  string
	projectID    string
	credentialID string
	privateKey   crypto.PrivateKey
	longitude    float64
	latitude     float64
	now          func() time.Time
}

// 天气响应
type weatherResponse struct {
	Location   json.RawMessage   `json:"location"`
	Current    json.RawMessage   `json:"current"`
	Minutely   json.RawMessage   `json:"minutely"`
	Hourly     json.RawMessage   `json:"hourly"`
	Daily      json.RawMessage   `json:"daily"`
	AirQuality json.RawMessage   `json:"air_quality"`
	Warning    json.RawMessage   `json:"warning"`
	Indices    json.RawMessage   `json:"indices"`
	Astronomy  astronomyResponse `json:"astronomy"`
}

// 天文响应
type astronomyResponse struct {
	Sun                 json.RawMessage `json:"sun"`
	Moon                json.RawMessage `json:"moon"`
	SolarElevationAngle json.RawMessage `json:"solar_elevation_angle"`
}

type requestSpec struct {
	name  string
	path  string
	query neturl.Values
}

type fetchResult struct {
	name string
	data json.RawMessage
	err  error
}

// 创建天气处理器
func NewHandler(cfg config.WeatherConfig) (*Handler, error) {
	if cfg.Longitude == nil || cfg.Latitude == nil {
		return nil, errors.New("weather coordinates are not configured")
	}

	privateKeyPEM, err := os.ReadFile(cfg.QWeatherPrivateKey)
	if err != nil {
		return nil, errors.New("failed to read QWeather private key")
	}
	privateKey, err := jwt.ParseEdPrivateKeyFromPEM(privateKeyPEM)
	if err != nil {
		return nil, errors.New("failed to parse QWeather private key")
	}

	return &Handler{
		client:       &http.Client{Timeout: requestTimeout},
		baseURL:      "https://" + strings.TrimSpace(cfg.QWeatherAPIHost),
		developerID:  strings.TrimSpace(cfg.QWeatherDeveloperID),
		projectID:    strings.TrimSpace(cfg.QWeatherProjectID),
		credentialID: strings.TrimSpace(cfg.QWeatherCredentialID),
		privateKey:   privateKey,
		longitude:    *cfg.Longitude,
		latitude:     *cfg.Latitude,
		now:          time.Now,
	}, nil
}

// 返回聚合天气数据
func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.Header().Set("Allow", http.MethodGet)
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	now := h.now()
	token, err := h.signToken(now)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to authorize weather request")
		return
	}

	longitude := strconv.FormatFloat(h.longitude, 'f', 2, 64)
	latitude := strconv.FormatFloat(h.latitude, 'f', 2, 64)
	coordinatePath := "/" + latitude + "/" + longitude
	legacyLocation := longitude + "," + latitude

	location, err := h.get(r.Context(), token, requestSpec{
		name: "location",
		path: "/geo/v2/city/lookup",
		query: neturl.Values{
			"location": []string{legacyLocation},
			"number":   []string{"1"},
		},
	})
	if err != nil {
		writeError(w, http.StatusBadGateway, "failed to fetch weather data")
		return
	}

	localTime, timezoneOffset, elevation, err := locationContext(location, now)
	if err != nil {
		writeError(w, http.StatusBadGateway, "failed to fetch weather data")
		return
	}
	date := localTime.Format("20060102")

	requests := []requestSpec{
		{name: "current", path: "/weather/v1/current" + coordinatePath},
		{name: "minutely", path: "/v7/minutely/5m", query: neturl.Values{"location": []string{legacyLocation}}},
		{name: "hourly", path: "/weather/v1/hourly" + coordinatePath, query: neturl.Values{"hours": []string{"240"}, "localTime": []string{"true"}}},
		{name: "daily", path: "/weather/v1/daily" + coordinatePath, query: neturl.Values{"days": []string{"10"}, "localTime": []string{"true"}}},
		{name: "air_quality", path: "/airquality/v1/current" + coordinatePath},
		{name: "warning", path: "/weatheralert/v1/current" + coordinatePath, query: neturl.Values{"localTime": []string{"true"}}},
		{name: "indices", path: "/v7/indices/3d", query: neturl.Values{"location": []string{legacyLocation}, "type": []string{"0"}}},
		{name: "sun", path: "/v7/astronomy/sun", query: neturl.Values{"location": []string{legacyLocation}, "date": []string{date}}},
		{name: "moon", path: "/v7/astronomy/moon", query: neturl.Values{"location": []string{legacyLocation}, "date": []string{date}}},
		{name: "solar_elevation_angle", path: "/v7/astronomy/solar-elevation-angle", query: neturl.Values{
			"location": []string{legacyLocation},
			"date":     []string{date},
			"time":     []string{localTime.Format("1504")},
			"tz":       []string{timezoneOffset},
			"alt":      []string{elevation},
		}},
	}

	data, err := h.getAll(r.Context(), token, requests)
	if err != nil {
		writeError(w, http.StatusBadGateway, "failed to fetch weather data")
		return
	}

	writeJSON(w, http.StatusOK, weatherResponse{
		Location:   location,
		Current:    data["current"],
		Minutely:   data["minutely"],
		Hourly:     data["hourly"],
		Daily:      data["daily"],
		AirQuality: data["air_quality"],
		Warning:    data["warning"],
		Indices:    data["indices"],
		Astronomy: astronomyResponse{
			Sun:                 data["sun"],
			Moon:                data["moon"],
			SolarElevationAngle: data["solar_elevation_angle"],
		},
	})
}

// 生成 JWT
func (h *Handler) signToken(now time.Time) (string, error) {
	issuedAt := now.Add(-jwtClockSkew).Unix()
	token := jwt.NewWithClaims(jwt.SigningMethodEdDSA, jwt.MapClaims{
		"iss": h.developerID,
		"sub": h.projectID,
		"iat": issuedAt,
		"exp": issuedAt + int64(jwtLifetime/time.Second),
	})
	delete(token.Header, "typ")
	token.Header["kid"] = h.credentialID
	return token.SignedString(h.privateKey)
}

// 并发请求上游接口
func (h *Handler) getAll(ctx context.Context, token string, requests []requestSpec) (map[string]json.RawMessage, error) {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	results := make(chan fetchResult, len(requests))
	for _, request := range requests {
		go func(request requestSpec) {
			data, err := h.get(ctx, token, request)
			results <- fetchResult{name: request.name, data: data, err: err}
		}(request)
	}

	data := make(map[string]json.RawMessage, len(requests))
	for range requests {
		result := <-results
		if result.err != nil {
			return nil, result.err
		}
		data[result.name] = result.data
	}
	return data, nil
}

// 请求和风接口
func (h *Handler) get(ctx context.Context, token string, request requestSpec) (json.RawMessage, error) {
	endpoint, err := neturl.Parse(h.baseURL + request.path)
	if err != nil {
		return nil, fmt.Errorf("%s request URL is invalid", request.name)
	}
	endpoint.RawQuery = request.query.Encode()

	httpRequest, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint.String(), nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create %s request", request.name)
	}
	httpRequest.Header.Set("Accept", "application/json")
	httpRequest.Header.Set("Authorization", "Bearer "+token)

	response, err := h.client.Do(httpRequest)
	if err != nil {
		return nil, fmt.Errorf("%s request failed", request.name)
	}
	defer response.Body.Close()

	body, err := io.ReadAll(io.LimitReader(response.Body, maxResponseSize+1))
	if err != nil {
		return nil, fmt.Errorf("failed to read %s response", request.name)
	}
	if len(body) > maxResponseSize {
		return nil, fmt.Errorf("%s response is too large", request.name)
	}
	if response.StatusCode < http.StatusOK || response.StatusCode >= http.StatusMultipleChoices {
		return nil, fmt.Errorf("%s returned HTTP status %d", request.name, response.StatusCode)
	}
	if !json.Valid(body) {
		return nil, fmt.Errorf("%s returned invalid JSON", request.name)
	}
	if code, ok := responseCode(body); ok && code != "200" {
		return nil, fmt.Errorf("%s returned API status %s", request.name, code)
	}
	return json.RawMessage(body), nil
}

// 解析地区上下文
func locationContext(data json.RawMessage, now time.Time) (time.Time, string, string, error) {
	var response struct {
		Location []struct {
			TZ        string `json:"tz"`
			UTCOffset string `json:"utcOffset"`
			Elevation string `json:"elevation"`
		} `json:"location"`
	}
	if err := json.Unmarshal(data, &response); err != nil || len(response.Location) == 0 {
		return time.Time{}, "", "", errors.New("location response does not contain a location")
	}

	location := response.Location[0]
	timezone, err := time.LoadLocation(location.TZ)
	if err != nil {
		parsed, parseErr := time.Parse("-07:00", location.UTCOffset)
		if parseErr != nil {
			return time.Time{}, "", "", errors.New("location response contains an invalid time zone")
		}
		_, offset := parsed.Zone()
		timezone = time.FixedZone(location.UTCOffset, offset)
	}

	localTime := now.In(timezone)
	elevation, err := strconv.ParseFloat(location.Elevation, 64)
	if err != nil {
		elevation = 0
	}
	return localTime,
		strings.TrimPrefix(localTime.Format("-0700"), "+"),
		strconv.FormatFloat(elevation, 'f', -1, 64),
		nil
}

// 解析上游状态码
func responseCode(data []byte) (string, bool) {
	var response struct {
		Code json.RawMessage `json:"code"`
	}
	if err := json.Unmarshal(data, &response); err != nil || len(response.Code) == 0 {
		return "", false
	}
	return strings.Trim(string(response.Code), `"`), true
}

// 输出错误 JSON
func writeError(w http.ResponseWriter, status int, message string) {
	writeJSON(w, status, map[string]string{"error": message})
}

// 输出 JSON
func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}
