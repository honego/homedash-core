package music

import (
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math/big"
	"net/http"
	"net/url"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/honeok/homedash-core/internal/config"
)

// 网易云听歌排行
const (
	playRecordURL   = "https://music.163.com/weapi/v1/play/record"
	requestTimeout  = 10 * time.Second
	maxResponseSize = 4 << 20
)

// 网易云 Web API 请求加密
const (
	weapiPresetKey      = "0CoJUm6Qyw8W8jud"
	weapiIV             = "0102030405060708"
	weapiPublicExponent = 0x10001
	weapiModulus        = "00e0b509f6259df8642dbc35662901477df22677ec152b5ff68ace615bb7" +
		"b725152b3ab17a876aea8a5aa76d2e417629ec4ee341f56135fccf695280" +
		"104e0312ecbda92557c93870114af6c9d05c4f7f0c3685b7a46bee255932" +
		"575cce10b424d813cfe4875d3e82047b97ddef52741d546b8e289dc6935b" +
		"3ece0462db0a22b8e7"
)

// 网易云客户端
type NetEase struct {
	client *http.Client
	userID int64
}

// 网易云音乐数据
type NetEaseData struct {
	Weekly []WeeklyTrack `json:"weekly"`
}

type playRecordResponse struct {
	Code     int `json:"code"`
	WeekData []struct {
		Song      neteaseSong `json:"song"`
		Score     int         `json:"score"`
		PlayCount int         `json:"playCount"`
	} `json:"weekData"`
}

type neteaseSong struct {
	ID         int64           `json:"id"`
	Name       string          `json:"name"`
	TNS        []string        `json:"tns"`
	Aliases    []string        `json:"alia"`
	Artists    []neteaseArtist `json:"ar"`
	Album      neteaseAlbum    `json:"al"`
	DurationMS int64           `json:"dt"`
	Disc       string          `json:"cd"`
	Track      int             `json:"no"`
}

type neteaseArtist struct {
	ID      int64    `json:"id"`
	Name    string   `json:"name"`
	TNS     []string `json:"tns"`
	Aliases []string `json:"alias"`
}

type neteaseAlbum struct {
	ID      int64    `json:"id"`
	Name    string   `json:"name"`
	TNS     []string `json:"tns"`
	Artwork string   `json:"picUrl"`
}

// 创建网易云客户端
func NewNetEase(cfg config.NetEaseConfig) *NetEase {
	return &NetEase{
		client: &http.Client{Timeout: requestTimeout},
		userID: cfg.UserID,
	}
}

// 获取一周听歌排行
func (n *NetEase) Get(ctx context.Context) (NetEaseData, error) {
	form, err := encryptWEAPI(map[string]any{"uid": n.userID, "type": 1})
	if err != nil {
		return NetEaseData{}, err
	}

	request, err := http.NewRequestWithContext(ctx, http.MethodPost, playRecordURL, strings.NewReader(form.Encode()))
	if err != nil {
		return NetEaseData{}, fmt.Errorf("failed to create NetEase request: %w", err)
	}
	request.Header.Set("Accept", "application/json")
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	response, err := n.client.Do(request)
	if err != nil {
		return NetEaseData{}, fmt.Errorf("NetEase request failed: %w", err)
	}
	defer response.Body.Close()

	body, err := io.ReadAll(io.LimitReader(response.Body, maxResponseSize+1))
	if err != nil {
		return NetEaseData{}, fmt.Errorf("failed to read NetEase response: %w", err)
	}
	if len(body) > maxResponseSize {
		return NetEaseData{}, errors.New("NetEase response is too large")
	}
	if response.StatusCode < http.StatusOK || response.StatusCode >= http.StatusMultipleChoices {
		return NetEaseData{}, fmt.Errorf("NetEase returned HTTP status %d", response.StatusCode)
	}
	if !json.Valid(body) {
		return NetEaseData{}, errors.New("NetEase returned invalid JSON")
	}

	var data playRecordResponse
	if err := json.Unmarshal(body, &data); err != nil {
		return NetEaseData{}, fmt.Errorf("failed to parse NetEase response: %w", err)
	}
	if data.Code != http.StatusOK {
		return NetEaseData{}, fmt.Errorf("NetEase returned API status %d", data.Code)
	}

	weekly := make([]WeeklyTrack, 0, len(data.WeekData))
	for _, item := range data.WeekData {
		weekly = append(weekly, WeeklyTrack{
			Track:     convertTrack(item.Song),
			Score:     item.Score,
			PlayCount: item.PlayCount,
		})
	}
	return NetEaseData{Weekly: weekly}, nil
}

func convertTrack(song neteaseSong) Track {
	artists := make([]Artist, 0, len(song.Artists))
	for _, artist := range song.Artists {
		artists = append(artists, Artist{
			ID:      strconv.FormatInt(artist.ID, 10),
			Name:    artist.Name,
			Aliases: mergeAliases(artist.TNS, artist.Aliases),
		})
	}

	discNumber, err := strconv.Atoi(strings.TrimSpace(song.Disc))
	if err != nil || discNumber < 1 {
		discNumber = 0
	}
	return Track{
		ID:      strconv.FormatInt(song.ID, 10),
		Title:   song.Name,
		Aliases: mergeAliases(song.TNS, song.Aliases),
		Artists: artists,
		Album: Album{
			ID:      strconv.FormatInt(song.Album.ID, 10),
			Title:   song.Album.Name,
			Aliases: mergeAliases(song.Album.TNS),
		},
		Artwork:     artworkURL(song.Album.Artwork),
		DurationMS:  song.DurationMS,
		DiscNumber:  discNumber,
		TrackNumber: song.Track,
	}
}

func mergeAliases(groups ...[]string) []string {
	aliases := make([]string, 0)
	seen := make(map[string]struct{})
	for _, group := range groups {
		for _, alias := range group {
			if _, ok := seen[alias]; ok {
				continue
			}
			seen[alias] = struct{}{}
			aliases = append(aliases, alias)
		}
	}
	return aliases
}

func artworkURL(value string) string {
	artwork, err := url.Parse(value)
	if err != nil || artwork.Host == "" || (artwork.Scheme != "http" && artwork.Scheme != "https") {
		return ""
	}
	artwork.Scheme = "https"
	return artwork.String()
}

func encryptWEAPI(value any) (url.Values, error) {
	data, err := json.Marshal(value)
	if err != nil {
		return nil, fmt.Errorf("failed to encode NetEase request: %w", err)
	}
	secret, err := randomSecret()
	if err != nil {
		return nil, err
	}
	params, err := aesEncrypt(data, []byte(weapiPresetKey))
	if err != nil {
		return nil, err
	}
	params, err = aesEncrypt([]byte(params), secret)
	if err != nil {
		return nil, err
	}
	encSecKey, err := rsaEncrypt(secret)
	if err != nil {
		return nil, err
	}
	return url.Values{
		"params":    []string{params},
		"encSecKey": []string{encSecKey},
	}, nil
}

func randomSecret() ([]byte, error) {
	const alphabet = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	secret := make([]byte, aes.BlockSize)
	limit := big.NewInt(int64(len(alphabet)))
	for i := range secret {
		value, err := rand.Int(rand.Reader, limit)
		if err != nil {
			return nil, fmt.Errorf("failed to generate NetEase secret: %w", err)
		}
		secret[i] = alphabet[value.Int64()]
	}
	return secret, nil
}

func aesEncrypt(data, key []byte) (string, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return "", fmt.Errorf("failed to create NetEase cipher: %w", err)
	}
	padding := aes.BlockSize - len(data)%aes.BlockSize
	padded := make([]byte, len(data)+padding)
	copy(padded, data)
	for i := len(data); i < len(padded); i++ {
		padded[i] = byte(padding)
	}

	encrypted := make([]byte, len(padded))
	cipher.NewCBCEncrypter(block, []byte(weapiIV)).CryptBlocks(encrypted, padded)
	return base64.StdEncoding.EncodeToString(encrypted), nil
}

func rsaEncrypt(secret []byte) (string, error) {
	modulus, ok := new(big.Int).SetString(weapiModulus, 16)
	if !ok {
		return "", errors.New("NetEase RSA modulus is invalid")
	}
	reversed := slices.Clone(secret)
	slices.Reverse(reversed)
	encrypted := new(big.Int).Exp(new(big.Int).SetBytes(reversed), big.NewInt(weapiPublicExponent), modulus)
	return fmt.Sprintf("%0256x", encrypted), nil
}
