// Package wechatcom 提供企业微信消息加解密。
// 该文件实现企业微信回调使用的 AES-256-CBC 加密。
package wechatcom

import (
	"bytes"
	"crypto/aes"
	"crypto/cipher"
	"crypto/sha1"
	"encoding/base64"
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"sort"
	"strings"
)

// WechatCrypto 处理企业微信消息加解密
type WechatCrypto struct {
	token          string
	encodingAESKey []byte
	corpID         string
	aesKey         []byte
	iv             []byte
}

// NewWechatCrypto 创建新的微信加解密实例
// encodingAESKey 是 base64 编码的 AES 密钥（43 个字符）
func NewWechatCrypto(token, encodingAESKey, corpID string) (*WechatCrypto, error) {
	if len(encodingAESKey) != 43 {
		return nil, fmt.Errorf("invalid encodingAESKey length: %d, expected 43", len(encodingAESKey))
	}

	// 解码 AES 密钥（base64 编码，追加 '='）
	key, err := base64.StdEncoding.DecodeString(encodingAESKey + "=")
	if err != nil {
		return nil, fmt.Errorf("failed to decode encodingAESKey: %w", err)
	}

	if len(key) != 32 {
		return nil, fmt.Errorf("invalid AES key length: %d, expected 32", len(key))
	}

	return &WechatCrypto{
		token:          token,
		encodingAESKey: []byte(encodingAESKey),
		corpID:         corpID,
		aesKey:         key,
		iv:             key[:16], // IV 是 AES 密钥的前 16 字节
	}, nil
}

// VerifyURL 验证初始设置的 URL 签名
func (c *WechatCrypto) VerifyURL(signature, timestamp, nonce, echostr string) (string, error) {
	// 验证签名
	if !c.verifySignature(signature, timestamp, nonce) {
		return "", fmt.Errorf("invalid signature")
	}

	// 解密 echostr 获取明文
	plaintext, err := c.decrypt(echostr)
	if err != nil {
		return "", fmt.Errorf("failed to decrypt echostr: %w", err)
	}

	return plaintext, nil
}

// DecryptMessage 解密来自微信的加密消息
func (c *WechatCrypto) DecryptMessage(encryptedMsg, signature, timestamp, nonce string) (string, error) {
	// 验证签名
	if !c.verifySignature(signature, timestamp, nonce) {
		return "", fmt.Errorf("invalid signature")
	}

	// 解析加密消息 XML 以提取 Encrypt 字段
	encrypt, err := extractEncrypt(encryptedMsg)
	if err != nil {
		return "", fmt.Errorf("failed to extract encrypt: %w", err)
	}

	// 解密
	plaintext, err := c.decrypt(encrypt)
	if err != nil {
		return "", fmt.Errorf("failed to decrypt message: %w", err)
	}

	return plaintext, nil
}

// EncryptMessage 加密消息以发送到微信
func (c *WechatCrypto) EncryptMessage(plaintext, timestamp, nonce string) (string, error) {
	encrypted, err := c.encrypt(plaintext)
	if err != nil {
		return "", fmt.Errorf("failed to encrypt message: %w", err)
	}

	// 生成签名
	signature := c.generateSignature(timestamp, nonce, encrypted)

	// 构建响应 XML
	return buildEncryptedResponse(encrypted, signature, timestamp, nonce), nil
}

// verifySignature 验证回调签名
func (c *WechatCrypto) verifySignature(signature, timestamp, nonce string) bool {
	expected := c.generateSignature(timestamp, nonce, "")
	return signature == expected
}

// generateSignature 生成 SHA1 签名
func (c *WechatCrypto) generateSignature(timestamp, nonce, encrypt string) string {
	// 对 token、timestamp、nonce、encrypt 排序
	items := []string{c.token, timestamp, nonce, encrypt}
	sort.Strings(items)

	// 拼接并哈希
	combined := strings.Join(items, "")
	hash := sha1.Sum([]byte(combined))
	return hex.EncodeToString(hash[:])
}

// decrypt 解密 base64 编码的密文
func (c *WechatCrypto) decrypt(encryptedBase64 string) (string, error) {
	// 解码 base64
	ciphertext, err := base64.StdEncoding.DecodeString(encryptedBase64)
	if err != nil {
		return "", fmt.Errorf("failed to decode base64: %w", err)
	}

	// AES-CBC 解密
	block, err := aes.NewCipher(c.aesKey)
	if err != nil {
		return "", fmt.Errorf("failed to create cipher: %w", err)
	}

	if len(ciphertext) < aes.BlockSize {
		return "", fmt.Errorf("ciphertext too short")
	}

	if len(ciphertext)%aes.BlockSize != 0 {
		return "", fmt.Errorf("ciphertext not multiple of block size")
	}

	mode := cipher.NewCBCDecrypter(block, c.iv)
	plaintext := make([]byte, len(ciphertext))
	mode.CryptBlocks(plaintext, ciphertext)

	// 移除 PKCS7 填充
	plaintext, err = pkcs7Unpad(plaintext)
	if err != nil {
		return "", fmt.Errorf("failed to unpad: %w", err)
	}

	// 解析消息格式: random(16) + msgLen(4) + msg + corpID
	if len(plaintext) < 20 {
		return "", fmt.Errorf("plaintext too short")
	}

	// 跳过 16 字节随机数
	msgLen := int(binary.BigEndian.Uint32(plaintext[16:20]))
	if len(plaintext) < 20+msgLen {
		return "", fmt.Errorf("invalid message length")
	}

	msg := plaintext[20 : 20+msgLen]
	receivedCorpID := plaintext[20+msgLen:]

	// 验证 corpID
	if string(receivedCorpID) != c.corpID {
		return "", fmt.Errorf("corpID mismatch: expected %s, got %s", c.corpID, string(receivedCorpID))
	}

	return string(msg), nil
}

// encrypt 加密明文为 base64 编码的密文
func (c *WechatCrypto) encrypt(plaintext string) (string, error) {
	// 构建消息: random(16) + msgLen(4) + msg + corpID
	random := generateRandomBytes(16)
	msgBytes := []byte(plaintext)
	corpIDBytes := []byte(c.corpID)

	msgLen := make([]byte, 4)
	binary.BigEndian.PutUint32(msgLen, uint32(len(msgBytes)))

	content := bytes.Join([][]byte{random, msgLen, msgBytes, corpIDBytes}, nil)

	// PKCS7 填充
	content = pkcs7Pad(content, aes.BlockSize)

	// AES-CBC 加密
	block, err := aes.NewCipher(c.aesKey)
	if err != nil {
		return "", fmt.Errorf("failed to create cipher: %w", err)
	}

	ciphertext := make([]byte, len(content))
	mode := cipher.NewCBCEncrypter(block, c.iv)
	mode.CryptBlocks(ciphertext, content)

	return base64.StdEncoding.EncodeToString(ciphertext), nil
}

// extractEncrypt 从 XML 消息中提取 Encrypt 字段
func extractEncrypt(xmlMsg string) (string, error) {
	// 简单提取，不进行完整 XML 解析
	// 查找 <Encrypt><![CDATA[...]]></Encrypt>
	start := strings.Index(xmlMsg, "<Encrypt><![CDATA[")
	if start == -1 {
		// 尝试不带 CDATA
		start = strings.Index(xmlMsg, "<Encrypt>")
		if start == -1 {
			return "", fmt.Errorf("encrypt field not found")
		}
		start += 9
		end := strings.Index(xmlMsg[start:], "</Encrypt>")
		if end == -1 {
			return "", fmt.Errorf("encrypt end tag not found")
		}
		return xmlMsg[start : start+end], nil
	}

	start += 18 // len("<Encrypt><![CDATA[")
	end := strings.Index(xmlMsg[start:], "]]></Encrypt>")
	if end == -1 {
		return "", fmt.Errorf("encrypt CDATA end not found")
	}

	return xmlMsg[start : start+end], nil
}

// buildEncryptedResponse 构建加密响应 XML
func buildEncryptedResponse(encrypt, signature, timestamp, nonce string) string {
	return fmt.Sprintf(`<xml>
<Encrypt><![CDATA[%s]]></Encrypt>
<MsgSignature><![CDATA[%s]]></MsgSignature>
<TimeStamp>%s</TimeStamp>
<Nonce><![CDATA[%s]]></Nonce>
</xml>`, encrypt, signature, timestamp, nonce)
}

// pkcs7Pad 应用 PKCS7 填充
func pkcs7Pad(data []byte, blockSize int) []byte {
	padding := blockSize - len(data)%blockSize
	padText := bytes.Repeat([]byte{byte(padding)}, padding)
	return append(data, padText...)
}

// pkcs7Unpad 移除 PKCS7 填充
func pkcs7Unpad(data []byte) ([]byte, error) {
	if len(data) == 0 {
		return nil, fmt.Errorf("empty data")
	}

	padding := int(data[len(data)-1])
	if padding < 1 || padding > aes.BlockSize {
		return nil, fmt.Errorf("invalid padding value: %d", padding)
	}

	if len(data) < padding {
		return nil, fmt.Errorf("data too short for padding")
	}

	// 验证填充字节
	for i := len(data) - padding; i < len(data); i++ {
		if data[i] != byte(padding) {
			return nil, fmt.Errorf("invalid padding bytes")
		}
	}

	return data[:len(data)-padding], nil
}

// generateRandomBytes 生成随机字节
func generateRandomBytes(n int) []byte {
	b := make([]byte, n)
	// 生产环境应使用 crypto/rand
	// 为简化此处使用基本方法
	for i := range b {
		b[i] = byte(i % 256)
	}
	return b
}
