package auth

import (
	"bytes"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"math/big"
	"net/http"
	"strconv"
	"strings"
	"time"

	"sql_platform/server/config"
)

/*
sso.go
----------------------------------------------------------------------
SSO（工作证）认证逻辑，全部在后端完成：
1. RSA 加密 token（使用后端配置的公钥）
2. 调用第三方 SSO API 验证 token
3. 解析返回结果获取用户名

前端只需将原始 token 传给后端，不再需要公钥、API 地址等敏感信息。
*/

// SSOVerifyResult SSO 验证结果
type SSOVerifyResult struct {
	Username string
	Success  bool
	Message  string
}

// VerifySSOToken 使用 RSA 加密 token 并调用 SSO API 验证
func VerifySSOToken(rawToken string, urlId string) (*SSOVerifyResult, error) {
	// log.Printf("[SSO] 原始token长度: %d 字节", len(rawToken))
	// log.Printf("[SSO] 原始token前30字符: %s", rawToken)

	// 1. RSA 加密 token
	encryptedToken, err := rsaEncryptLong(config.SSOPublicKey, rawToken)
	if err != nil {
		return nil, fmt.Errorf("RSA加密失败: %v", err)
	}

	// log.Printf("[SSO] 加密后token长度: %d", len(encryptedToken))
	// if len(encryptedToken) > 50 {
	// 	log.Printf("[SSO] 加密后token前50字符: %s", encryptedToken[:50])
	// }

	// 2. 调用 SSO API
	// urlId 转为数字（SSO API 要求数字类型）
	urlIdNum, err := strconv.Atoi(urlId)
	if err != nil {
		urlIdNum = 0
	}

	payload := map[string]interface{}{
		"token": encryptedToken,
		"urlId": urlIdNum,
	}
	payloadBytes, _ := json.Marshal(payload)
	// log.Printf("[SSO] 发送的JSON payload: %s", string(payloadBytes))

	// 使用 http.NewRequest 显式设置请求头，模拟浏览器 fetch 的行为
	req, err := http.NewRequest("POST", config.SSOGzzApiUrl, bytes.NewBuffer(payloadBytes))
	if err != nil {
		return nil, fmt.Errorf("创建SSO请求失败: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36")

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("调用SSO接口失败: %v", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("读取SSO响应失败: %v", err)
	}

	// log.Printf("[SSO] 请求地址: %s", config.SSOGzzApiUrl)
	// log.Printf("[SSO] urlId: %d", urlIdNum)
	// log.Printf("[SSO] 响应: %s", string(body))

	// 3. 解析 SSO 返回
	var ssoResp map[string]interface{}
	if err := json.Unmarshal(body, &ssoResp); err != nil {
		return nil, fmt.Errorf("解析SSO响应失败: %v", err)
	}

	// 检查 status（0 表示成功，可能是数字或字符串）
	statusVal, hasStatus := ssoResp["status"]
	if hasStatus {
		status := fmt.Sprintf("%v", statusVal)
		if status != "0" {
			msg := ""
			if m, ok := ssoResp["msg"]; ok {
				msg = fmt.Sprintf("%v", m)
			}
			return &SSOVerifyResult{Success: false, Message: "第三方校验失败：" + msg}, nil
		}
	}

	// 4. 解析 username
	username := parseSSOUsername(ssoResp["result"])
	username = strings.TrimSpace(username)

	if username == "" {
		return &SSOVerifyResult{Success: false, Message: "从第三方返回报文中解析用户名失败"}, nil
	}

	return &SSOVerifyResult{Username: username, Success: true}, nil
}

// parseSSOUsername 从 SSO 返回的 result 字段中提取用户名
func parseSSOUsername(result interface{}) string {
	if result == nil {
		return ""
	}
	switch v := result.(type) {
	case string:
		if strings.Contains(v, "username=") {
			parts := strings.SplitN(v, "username=", 2)
			if len(parts) > 1 {
				return strings.SplitN(parts[1], "&", 2)[0]
			}
		}
		if strings.Contains(v, "=") {
			parts := strings.SplitN(v, "=", 2)
			if len(parts) > 1 {
				return parts[1]
			}
		}
		return v
	case map[string]interface{}:
		if u, ok := v["username"]; ok {
			return fmt.Sprintf("%v", u)
		}
	}
	return ""
}

// rsaEncryptLong 使用 RSA 公钥加密长文本（分块加密 + base64 拼接）
// 与 JSEncrypt 的 encryptLong 行为一致
func rsaEncryptLong(publicKeyBase64 string, plaintext string) (string, error) {
	// 解码 base64 公钥
	keyBytes, err := base64.StdEncoding.DecodeString(publicKeyBase64)
	if err != nil {
		return "", fmt.Errorf("公钥base64解码失败: %v", err)
	}

	// 尝试 PKIX 格式解析
	pub, err := x509.ParsePKIXPublicKey(keyBytes)
	if err != nil {
		// 尝试 PKCS1 格式
		pubKey, err2 := x509.ParsePKCS1PublicKey(keyBytes)
		if err2 != nil {
			return "", fmt.Errorf("公钥解析失败: %v / %v", err, err2)
		}
		pub = pubKey
	}

	rsaPub, ok := pub.(*rsa.PublicKey)
	if !ok {
		return "", fmt.Errorf("不是RSA公钥")
	}

	// 分块大小 = 密钥字节数 - 11（PKCS1v15 填充开销）
	keySize := rsaPub.Size()
	chunkSize := keySize - 11
	if chunkSize <= 0 {
		return "", fmt.Errorf("密钥太小")
	}

	// 与 JSEncrypt encryptLong 一致：
	// 1. 按 117 字符分块（对 ASCII 等同于按字节分块）
	// 2. 每块 RSA 加密 → 转 HEX 字符串
	// 3. 拼接所有 HEX 字符串
	// 4. 最后一次性把整个 HEX 转为 base64
	plaintextRunes := []rune(plaintext)
	// numChunks := (len(plaintextRunes) + chunkSize - 1) / chunkSize
	// log.Printf("[SSO] RSA密钥大小: %d 字节, 分块大小: %d, 原文长度: %d 字符, 分块数: %d", keySize, chunkSize, len(plaintextRunes), numChunks)

	var hexResult strings.Builder

	for i := 0; i < len(plaintextRunes); i += chunkSize {
		end := i + chunkSize
		if end > len(plaintextRunes) {
			end = len(plaintextRunes)
		}

		// 取出这一块字符，转为字节
		chunkStr := string(plaintextRunes[i:end])
		chunk := []byte(chunkStr)
		encrypted, err := rsa.EncryptPKCS1v15(rand.Reader, rsaPub, chunk)
		if err != nil {
			return "", fmt.Errorf("加密分块失败: %v", err)
		}

		// 每块加密结果转 HEX（与 JSEncrypt 的 doPublic(m).toString(16) 一致，去掉前导零）
		n := new(big.Int).SetBytes(encrypted)
		hexResult.WriteString(fmt.Sprintf("%x", n))
	}

	// 把整个拼接后的 HEX 字符串一次性转为 base64（与 hex2b64 一致）
	hexStr := hexResult.String()
	// log.Printf("[SSO] 拼接HEX总长度: %d, HEX前30字符: %s", len(hexStr), hexStr[:min(30, len(hexStr))])

	// hex2b64: 将 hex 字符串转为 base64
	return hexToBase64(hexStr), nil
}

// hexToBase64 将 hex 字符串转为 base64（与 JSEncrypt 的 hex2b64 函数行为一致）
func hexToBase64(h string) string {
	bytes, err := hex.DecodeString(h)
	if err != nil {
		// fallback: 标准库解码失败时手动处理
		return base64.StdEncoding.EncodeToString([]byte(h))
	}
	return base64.StdEncoding.EncodeToString(bytes)
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
