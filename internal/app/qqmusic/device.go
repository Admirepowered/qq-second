package qqmusic

// 模拟 Android 设备身份与 QIMEI 指纹申请（对应 qm-api-rs device.rs / qimei.rs）。

import (
	"bytes"
	"crypto/aes"
	"crypto/cipher"
	"crypto/md5"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

const qimeiPublicKey = `MIGfMA0GCSqGSIb3DQEBAQUAA4GNADCBiQKBgQDEIxgwoutfwoJxcGQeedgP7FG9qaIuS0qzfR8gWkrkTZKM2iWHn2ajQpBRZjMSoSf6+KJGvar2ORhBfpDXyVtZCKpqLQ+FLkpncClKVIrBwv6PHyUvuCb0rIarmgDnzkfQAqVufEtR64iazGDKatvJ9y6B9NMbHddGSAUmRTCrHQIDAQAB`

const (
	qimeiSecret    = "ZdJqM15EeO2zWc08"
	qimeiAppKey    = "0AND0HD6FE4HY80F"
	qimeiChannelID = "10003505"
	qimeiPackageID = "com.tencent.qqmusic"
)

// OSVersion 模拟设备的系统版本。
type OSVersion struct {
	Incremental string
	Release     string
	Codename    string
	SDK         int
}

// Device 是一台随机模拟设备。
type Device struct {
	Display      string
	Product      string
	Device       string
	Board        string
	Model        string
	Fingerprint  string
	BootID       string
	ProcVersion  string
	IMEI         string
	Brand        string
	Bootloader   string
	BaseBand     string
	Version      OSVersion
	SimInfo      string
	OSType       string
	MacAddress   string
	WifiBSSID    string
	WifiSSID     string
	IMSIMD5      []byte
	AndroidID    string
	APN          string
	VendorName   string
	VendorOSName string
	Qimei        string
	Qimei36      string
	QimeiSaveAt  int64
	OpenUDID     string
}

func randHex(n int) string {
	b := make([]byte, n)
	_, _ = rand.Read(b)
	const chars = "0123456789abcdef"
	out := make([]byte, n)
	for i := range b {
		out[i] = chars[int(b[i])%16]
	}
	return string(out)
}

func randomIMEI() string {
	digits := make([]int, 14)
	b := make([]byte, 14)
	_, _ = rand.Read(b)
	for i := range digits {
		digits[i] = int(b[i]) % 10
	}
	sum := 0
	for idx, d := range digits {
		checksum := d
		if idx%2 == 1 {
			checksum *= 2
			if checksum > 9 {
				checksum -= 9
			}
		}
		sum += checksum
	}
	ctrl := (10 - sum%10) % 10
	digits = append(digits, ctrl)
	var sb strings.Builder
	for _, d := range digits {
		sb.WriteByte('0' + byte(d))
	}
	return sb.String()
}

func uuidV4() string {
	b := make([]byte, 16)
	_, _ = rand.Read(b)
	b[6] = (b[6] & 0x0f) | 0x40
	b[8] = (b[8] & 0x3f) | 0x80
	return fmt.Sprintf("%x-%x-%x-%x-%x", b[0:4], b[4:6], b[6:8], b[8:10], b[10:16])
}

// RandomDevice 生成一台随机模拟设备。
func RandomDevice() *Device {
	imsiSeed := make([]byte, 16)
	_, _ = rand.Read(imsiSeed)
	imsiSum := md5.Sum(imsiSeed)

	fingerprintSeq := randRange(1000000, 9999999)
	displaySeq := randRange(100000, 999999)

	return &Device{
		Display:      fmt.Sprintf("QMAPI.%d.001", displaySeq),
		Product:      "iarim",
		Device:       "sagit",
		Board:        "eomam",
		Model:        "MI 6",
		Fingerprint:  fmt.Sprintf("xiaomi/iarim/sagit:10/eomam.200122.001/%d:user/release-keys", fingerprintSeq),
		BootID:       uuidV4(),
		ProcVersion:  fmt.Sprintf("Linux 5.4.0-54-generic-%s (android-build@google.com)", randHex(8)),
		IMEI:         randomIMEI(),
		Brand:        "Xiaomi",
		Bootloader:   "U-boot",
		Version:      OSVersion{Incremental: "5891938", Release: "10", Codename: "REL", SDK: 29},
		SimInfo:      "T-Mobile",
		OSType:       "android",
		MacAddress:   "00:50:56:C0:00:08",
		WifiBSSID:    "00:50:56:C0:00:08",
		WifiSSID:     "<unknown ssid>",
		IMSIMD5:      imsiSum[:],
		AndroidID:    randHex(8),
		APN:          "wifi",
		VendorName:   "MIUI",
		VendorOSName: "qmapi",
		OpenUDID:     uuidSimple(),
	}
}

func uuidSimple() string {
	b := make([]byte, 16)
	_, _ = rand.Read(b)
	b[6] = (b[6] & 0x0f) | 0x40
	b[8] = (b[8] & 0x3f) | 0x80
	return hex.EncodeToString(b)
}

// randRange 返回 [min, max) 内的随机整数。
func randRange(min, max int64) int64 {
	b := make([]byte, 8)
	_, _ = rand.Read(b)
	var v int64
	for _, x := range b {
		v = v<<8 | int64(x)
	}
	if v < 0 {
		v = -v
	}
	return min + v%(max-min)
}

func nowSecs() int64 { return time.Now().Unix() }

func randomBeaconID() string {
	now := time.Now().UTC()
	timeMonth := fmt.Sprintf("%d-%02d-01", now.Year(), int(now.Month()))
	rand1 := randRange(100000, 999999)
	rand2 := randRange(100000000, 999999999)
	var sb strings.Builder
	for i := 1; i <= 40; i++ {
		switch i {
		case 1, 2, 13, 14, 17, 18, 21, 22, 25, 26, 29, 30, 33, 34, 37, 38:
			sb.WriteString(fmt.Sprintf("k%d:%s%d.%d", i, timeMonth, rand1, rand2))
		case 3:
			sb.WriteString("k3:0000000000000000")
		case 4:
			sb.WriteString(fmt.Sprintf("k4:%s", randHex(16)))
		default:
			sb.WriteString(fmt.Sprintf("k%d:%d", i, randRange(0, 9999)))
		}
		sb.WriteByte(';')
	}
	return sb.String()
}

func randomPayloadByDevice(device *Device, version, sdkVersion string) map[string]interface{} {
	fixedRand := randRange(0, 14400)
	uptime := time.Now().UTC().Add(-time.Duration(fixedRand) * time.Second)
	reserved := map[string]interface{}{
		"harmony":    "0",
		"clone":      "0",
		"containe":   "",
		"oz":         "UhYmelwouA+V2nPWbOvLTgN2/m8jwGB+yUB5v9tysQg=",
		"oo":         "Xecjt+9S1+f8Pz2VLSxgpw==",
		"kelong":     "0",
		"uptimes":    uptime.Format("2006-01-02 15:04:05"),
		"multiUser":  "0",
		"bod":        device.Brand,
		"dv":         device.Device,
		"firstLevel": "",
		"manufact":   device.Brand,
		"name":       device.Model,
		"host":       "se.infra",
		"kernel":     device.ProcVersion,
	}
	reservedJSON, _ := json.Marshal(reserved)
	return map[string]interface{}{
		"androidId":        device.AndroidID,
		"platformId":       1,
		"appKey":           qimeiAppKey,
		"appVersion":       version,
		"beaconIdSrc":      randomBeaconID(),
		"brand":            device.Brand,
		"channelId":        qimeiChannelID,
		"cid":              "",
		"imei":             device.IMEI,
		"imsi":             "",
		"mac":              "",
		"model":            device.Model,
		"networkType":      "unknown",
		"oaid":             "",
		"osVersion":        fmt.Sprintf("Android %s,level %d", device.Version.Release, device.Version.SDK),
		"qimei":            "",
		"qimei36":          "",
		"sdkVersion":       sdkVersion,
		"targetSdkVersion": "33",
		"audit":            "",
		"userId":           "{}",
		"packageId":        qimeiPackageID,
		"deviceType":       "Phone",
		"sdkName":          "",
		"reserved":         string(reservedJSON),
	}
}

func rsaEncryptPKCS1(content []byte) ([]byte, error) {
	der, err := base64.StdEncoding.DecodeString(qimeiPublicKey)
	if err != nil {
		return nil, err
	}
	pub, err := x509.ParsePKIXPublicKey(der)
	if err != nil {
		return nil, err
	}
	rsaPub, ok := pub.(*rsa.PublicKey)
	if !ok {
		return nil, fmt.Errorf("not RSA key")
	}
	return rsa.EncryptPKCS1v15(rand.Reader, rsaPub, content)
}

func aesEncryptCBC(key, content []byte) ([]byte, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	padded := pkcs7Pad(content, aes.BlockSize)
	out := make([]byte, len(padded))
	iv := key[:aes.BlockSize]
	cipher.NewCBCEncrypter(block, iv).CryptBlocks(out, padded)
	return out, nil
}

func pkcs7Pad(data []byte, blockSize int) []byte {
	pad := blockSize - len(data)%blockSize
	return append(data, bytes.Repeat([]byte{byte(pad)}, pad)...)
}

func calcMD5(parts ...[]byte) string {
	h := md5.New()
	for _, p := range parts {
		h.Write(p)
	}
	return hex.EncodeToString(h.Sum(nil))
}

// fetchQimei 向 api.tencentmusic.com 申请 QIMEI 指纹。
func fetchQimei(device *Device, httpClient *http.Client) (string, string, error) {
	payload, _ := json.Marshal(randomPayloadByDevice(device, "14.9.0.8", "1.2.13.6"))
	cryptKey := randHex(16)
	nonce := randHex(16)
	ts := nowSecs()

	rsaOut, err := rsaEncryptPKCS1([]byte(cryptKey))
	if err != nil {
		return "", "", err
	}
	aesOut, err := aesEncryptCBC([]byte(cryptKey), payload)
	if err != nil {
		return "", "", err
	}
	keyB64 := base64.StdEncoding.EncodeToString(rsaOut)
	paramsB64 := base64.StdEncoding.EncodeToString(aesOut)
	extra := fmt.Sprintf(`{"appKey":"%s"}`, qimeiAppKey)

	reqSign := calcMD5([]byte(keyB64), []byte(paramsB64), []byte(fmt.Sprintf("%d", ts*1000)), []byte(nonce), []byte(qimeiSecret), []byte(extra))
	headerSign := calcMD5([]byte("qimei_qq_androidpzAuCmaFAaFaHrdakPjLIEqKrGnSOOvH"), []byte(fmt.Sprintf("%d", ts)))

	body := map[string]interface{}{
		"app": 0,
		"os":  1,
		"qimeiParams": map[string]interface{}{
			"key":    keyB64,
			"params": paramsB64,
			"time":   fmt.Sprintf("%d", ts),
			"nonce":  nonce,
			"sign":   reqSign,
			"extra":  extra,
		},
	}
	bodyJSON, _ := json.Marshal(body)

	req, err := http.NewRequest(http.MethodPost, "https://api.tencentmusic.com/tme/trpc/proxy", bytes.NewReader(bodyJSON))
	if err != nil {
		return "", "", err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("method", "GetQimei")
	req.Header.Set("service", "trpc.tme_datasvr.qimeiproxy.QimeiProxy")
	req.Header.Set("appid", "qimei_qq_android")
	req.Header.Set("sign", headerSign)
	req.Header.Set("User-Agent", "QQMusic")
	req.Header.Set("timestamp", fmt.Sprintf("%d", ts))

	resp, err := httpClient.Do(req)
	if err != nil {
		return "", "", err
	}
	defer resp.Body.Close()
	raw, err := io.ReadAll(io.LimitReader(resp.Body, 4<<20))
	if err != nil {
		return "", "", err
	}
	var outer map[string]interface{}
	if err := json.Unmarshal(raw, &outer); err != nil {
		return "", "", err
	}
	dataStr, _ := outer["data"].(string)
	var inner map[string]interface{}
	if err := json.Unmarshal([]byte(dataStr), &inner); err != nil {
		return "", "", fmt.Errorf("qimei inner parse: %w", err)
	}
	dataMap, _ := inner["data"].(map[string]interface{})
	if dataMap == nil {
		return "", "", fmt.Errorf("qimei response missing data")
	}
	q16, _ := dataMap["q16"].(string)
	q36, _ := dataMap["q36"].(string)
	if q16 == "" || q36 == "" {
		return "", "", fmt.Errorf("qimei response missing q16/q36")
	}
	return q16, q36, nil
}
