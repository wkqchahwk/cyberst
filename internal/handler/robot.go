package handler

import (
	"bytes"
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha1"
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"net/http"
	"sort"
	"strings"
	"sync"
	"time"

	"cyberstrike-ai/internal/config"
	"cyberstrike-ai/internal/database"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

const (
	robotCmdHelp        = ""
	robotCmdList        = ""
	robotCmdListAlt     = ""
	robotCmdSwitch      = ""
	robotCmdContinue    = ""
	robotCmdNew         = ""
	robotCmdClear       = ""
	robotCmdCurrent     = ""
	robotCmdStop        = ""
	robotCmdRoles       = ""
	robotCmdRolesList   = ""
	robotCmdSwitchRole  = ""
	robotCmdDelete      = ""
	robotCmdVersion     = ""
)

// English note.
type RobotHandler struct {
	config         *config.Config
	db             *database.DB
	agentHandler   *AgentHandler
	logger         *zap.Logger
	mu             sync.RWMutex
	sessions       map[string]string             // key: "platform_userID", value: conversationID
	sessionRoles   map[string]string             // key: "platform_userID", value: roleName（""）
	cancelMu       sync.Mutex                    //  runningCancels
	runningCancels map[string]context.CancelFunc // key: "platform_userID", 
}

// English note.
func NewRobotHandler(cfg *config.Config, db *database.DB, agentHandler *AgentHandler, logger *zap.Logger) *RobotHandler {
	return &RobotHandler{
		config:         cfg,
		db:             db,
		agentHandler:   agentHandler,
		logger:         logger,
		sessions:       make(map[string]string),
		sessionRoles:   make(map[string]string),
		runningCancels: make(map[string]context.CancelFunc),
	}
}

// English note.
func (h *RobotHandler) sessionKey(platform, userID string) string {
	return platform + "_" + userID
}

// English note.
func (h *RobotHandler) getOrCreateConversation(platform, userID, title string) (convID string, isNew bool) {
	h.mu.RLock()
	convID = h.sessions[h.sessionKey(platform, userID)]
	h.mu.RUnlock()
	if convID != "" {
		return convID, false
	}
	t := strings.TrimSpace(title)
	if t == "" {
		t = " " + time.Now().Format("01-02 15:04")
	} else {
		t = safeTruncateString(t, 50)
	}
	conv, err := h.db.CreateConversation(t)
	if err != nil {
		h.logger.Warn("", zap.Error(err))
		return "", false
	}
	convID = conv.ID
	h.mu.Lock()
	h.sessions[h.sessionKey(platform, userID)] = convID
	h.mu.Unlock()
	return convID, true
}

// English note.
func (h *RobotHandler) setConversation(platform, userID, convID string) {
	h.mu.Lock()
	h.sessions[h.sessionKey(platform, userID)] = convID
	h.mu.Unlock()
}

// English note.
func (h *RobotHandler) getRole(platform, userID string) string {
	h.mu.RLock()
	role := h.sessionRoles[h.sessionKey(platform, userID)]
	h.mu.RUnlock()
	if role == "" {
		return ""
	}
	return role
}

// English note.
func (h *RobotHandler) setRole(platform, userID, roleName string) {
	h.mu.Lock()
	h.sessionRoles[h.sessionKey(platform, userID)] = roleName
	h.mu.Unlock()
}

// English note.
func (h *RobotHandler) clearConversation(platform, userID string) (newConvID string) {
	title := " " + time.Now().Format("01-02 15:04")
	conv, err := h.db.CreateConversation(title)
	if err != nil {
		h.logger.Warn("", zap.Error(err))
		return ""
	}
	h.setConversation(platform, userID, conv.ID)
	return conv.ID
}

// English note.
func (h *RobotHandler) HandleMessage(platform, userID, text string) (reply string) {
	text = strings.TrimSpace(text)
	if text == "" {
		return "「」/ help 。"
	}

	// English note.
	if cmdReply, ok := h.handleRobotCommand(platform, userID, text); ok {
		return cmdReply
	}

	// English note.
	convID, _ := h.getOrCreateConversation(platform, userID, text)
	if convID == "" {
		return "，。"
	}
	// English note.
	if conv, err := h.db.GetConversation(convID); err == nil && strings.HasPrefix(conv.Title, " ") {
		newTitle := safeTruncateString(text, 50)
		if newTitle != "" {
			_ = h.db.UpdateConversationTitle(convID, newTitle)
		}
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	sk := h.sessionKey(platform, userID)
	h.cancelMu.Lock()
	h.runningCancels[sk] = cancel
	h.cancelMu.Unlock()
	defer func() {
		cancel()
		h.cancelMu.Lock()
		delete(h.runningCancels, sk)
		h.cancelMu.Unlock()
	}()
	role := h.getRole(platform, userID)
	resp, newConvID, err := h.agentHandler.ProcessMessageForRobot(ctx, convID, text, role)
	if err != nil {
		h.logger.Warn(" Agent ", zap.String("platform", platform), zap.String("userID", userID), zap.Error(err))
		if errors.Is(err, context.Canceled) {
			return "。"
		}
		return ": " + err.Error()
	}
	if newConvID != convID {
		h.setConversation(platform, userID, newConvID)
	}
	return resp
}

func (h *RobotHandler) cmdHelp() string {
	return "**【CyberStrikeAI 】**\n\n" +
		"- `` `help` —  | Show this help\n" +
		"- `` `list` —  ID | List conversations\n" +
		"- ` <ID>` `switch <ID>` —  | Switch to conversation\n" +
		"- `` `new` —  | Start new conversation\n" +
		"- `` `clear` —  | Clear context\n" +
		"- `` `current` —  ID  | Show current conversation\n" +
		"- `` `stop` —  | Stop running task\n" +
		"- `` `roles` —  | List roles\n" +
		"- ` <>` `role <name>` —  | Switch role\n" +
		"- ` <ID>` `delete <ID>` —  | Delete conversation\n" +
		"- `` `version` —  | Show version\n\n" +
		"---\n" +
		"， AI /。\n" +
		"Otherwise, send any text for AI penetration testing / security analysis."
}

func (h *RobotHandler) cmdList() string {
	convs, err := h.db.ListConversations(50, 0, "")
	if err != nil {
		return ": " + err.Error()
	}
	if len(convs) == 0 {
		return "。。"
	}
	var b strings.Builder
	b.WriteString("【】\n")
	for i, c := range convs {
		if i >= 20 {
			b.WriteString("…  20 \n")
			break
		}
		b.WriteString(fmt.Sprintf("· %s\n  ID: %s\n", c.Title, c.ID))
	}
	return strings.TrimSuffix(b.String(), "\n")
}

func (h *RobotHandler) cmdSwitch(platform, userID, convID string) string {
	if convID == "" {
		return " ID，： xxx-xxx-xxx"
	}
	conv, err := h.db.GetConversation(convID)
	if err != nil {
		return " ID 。"
	}
	h.setConversation(platform, userID, conv.ID)
	return fmt.Sprintf("：「%s」\nID: %s", conv.Title, conv.ID)
}

func (h *RobotHandler) cmdNew(platform, userID string) string {
	newID := h.clearConversation(platform, userID)
	if newID == "" {
		return "，。"
	}
	return "，。"
}

func (h *RobotHandler) cmdClear(platform, userID string) string {
	return h.cmdNew(platform, userID)
}

func (h *RobotHandler) cmdStop(platform, userID string) string {
	sk := h.sessionKey(platform, userID)
	h.cancelMu.Lock()
	cancel, ok := h.runningCancels[sk]
	if ok {
		delete(h.runningCancels, sk)
		cancel()
	}
	h.cancelMu.Unlock()
	if !ok {
		return "。"
	}
	return "。"
}

func (h *RobotHandler) cmdCurrent(platform, userID string) string {
	h.mu.RLock()
	convID := h.sessions[h.sessionKey(platform, userID)]
	h.mu.RUnlock()
	if convID == "" {
		return "。。"
	}
	conv, err := h.db.GetConversation(convID)
	if err != nil {
		return " ID: " + convID + "（）"
	}
	role := h.getRole(platform, userID)
	return fmt.Sprintf("：「%s」\nID: %s\n: %s", conv.Title, conv.ID, role)
}

func (h *RobotHandler) cmdRoles() string {
	if h.config.Roles == nil || len(h.config.Roles) == 0 {
		return "。"
	}
	names := make([]string, 0, len(h.config.Roles))
	for name, role := range h.config.Roles {
		if role.Enabled {
			names = append(names, name)
		}
	}
	if len(names) == 0 {
		return "。"
	}
	sort.Slice(names, func(i, j int) bool {
		if names[i] == "" {
			return true
		}
		if names[j] == "" {
			return false
		}
		return names[i] < names[j]
	})
	var b strings.Builder
	b.WriteString("【】\n")
	for _, name := range names {
		role := h.config.Roles[name]
		desc := role.Description
		if desc == "" {
			desc = ""
		}
		b.WriteString(fmt.Sprintf("· %s — %s\n", name, desc))
	}
	return strings.TrimSuffix(b.String(), "\n")
}

func (h *RobotHandler) cmdSwitchRole(platform, userID, roleName string) string {
	if roleName == "" {
		return "，： "
	}
	if h.config.Roles == nil {
		return "。"
	}
	role, exists := h.config.Roles[roleName]
	if !exists {
		return fmt.Sprintf("「%s」。「」。", roleName)
	}
	if !role.Enabled {
		return fmt.Sprintf("「%s」。", roleName)
	}
	h.setRole(platform, userID, roleName)
	return fmt.Sprintf("：「%s」\n%s", roleName, role.Description)
}

func (h *RobotHandler) cmdDelete(platform, userID, convID string) string {
	if convID == "" {
		return " ID，： xxx-xxx-xxx"
	}
	sk := h.sessionKey(platform, userID)
	h.mu.RLock()
	currentConvID := h.sessions[sk]
	h.mu.RUnlock()
	if convID == currentConvID {
		// English note.
		h.mu.Lock()
		delete(h.sessions, sk)
		h.mu.Unlock()
	}
	if err := h.db.DeleteConversation(convID); err != nil {
		return ": " + err.Error()
	}
	return fmt.Sprintf(" ID: %s", convID)
}

func (h *RobotHandler) cmdVersion() string {
	v := h.config.Version
	if v == "" {
		v = ""
	}
	return "CyberStrikeAI " + v
}

// English note.
func (h *RobotHandler) handleRobotCommand(platform, userID, text string) (string, bool) {
	switch {
	case text == robotCmdHelp || text == "help" || text == "？" || text == "?":
		return h.cmdHelp(), true
	case text == robotCmdList || text == robotCmdListAlt || text == "list":
		return h.cmdList(), true
	case strings.HasPrefix(text, robotCmdSwitch+" ") || strings.HasPrefix(text, robotCmdContinue+" ") || strings.HasPrefix(text, "switch ") || strings.HasPrefix(text, "continue "):
		var id string
		switch {
		case strings.HasPrefix(text, robotCmdSwitch+" "):
			id = strings.TrimSpace(text[len(robotCmdSwitch)+1:])
		case strings.HasPrefix(text, robotCmdContinue+" "):
			id = strings.TrimSpace(text[len(robotCmdContinue)+1:])
		case strings.HasPrefix(text, "switch "):
			id = strings.TrimSpace(text[7:])
		default:
			id = strings.TrimSpace(text[9:])
		}
		return h.cmdSwitch(platform, userID, id), true
	case text == robotCmdNew || text == "new":
		return h.cmdNew(platform, userID), true
	case text == robotCmdClear || text == "clear":
		return h.cmdClear(platform, userID), true
	case text == robotCmdCurrent || text == "current":
		return h.cmdCurrent(platform, userID), true
	case text == robotCmdStop || text == "stop":
		return h.cmdStop(platform, userID), true
	case text == robotCmdRoles || text == robotCmdRolesList || text == "roles":
		return h.cmdRoles(), true
	case strings.HasPrefix(text, robotCmdRoles+" ") || strings.HasPrefix(text, robotCmdSwitchRole+" ") || strings.HasPrefix(text, "role "):
		var roleName string
		switch {
		case strings.HasPrefix(text, robotCmdRoles+" "):
			roleName = strings.TrimSpace(text[len(robotCmdRoles)+1:])
		case strings.HasPrefix(text, robotCmdSwitchRole+" "):
			roleName = strings.TrimSpace(text[len(robotCmdSwitchRole)+1:])
		default:
			roleName = strings.TrimSpace(text[5:])
		}
		return h.cmdSwitchRole(platform, userID, roleName), true
	case strings.HasPrefix(text, robotCmdDelete+" ") || strings.HasPrefix(text, "delete "):
		var convID string
		if strings.HasPrefix(text, robotCmdDelete+" ") {
			convID = strings.TrimSpace(text[len(robotCmdDelete)+1:])
		} else {
			convID = strings.TrimSpace(text[7:])
		}
		return h.cmdDelete(platform, userID, convID), true
	case text == robotCmdVersion || text == "version":
		return h.cmdVersion(), true
	default:
		return "", false
	}
}

// English note.

// English note.
type wecomXML struct {
	ToUserName   string `xml:"ToUserName"`
	FromUserName string `xml:"FromUserName"`
	CreateTime   int64  `xml:"CreateTime"`
	MsgType      string `xml:"MsgType"`
	Content      string `xml:"Content"`
	MsgID        string `xml:"MsgId"`
	AgentID      int64  `xml:"AgentID"`
	Encrypt      string `xml:"Encrypt"` // 
}

// English note.
type wecomReplyXML struct {
	XMLName      xml.Name `xml:"xml"`
	ToUserName   string   `xml:"ToUserName"`
	FromUserName string   `xml:"FromUserName"`
	CreateTime   int64    `xml:"CreateTime"`
	MsgType      string   `xml:"MsgType"`
	Content      string   `xml:"Content"`
}

// English note.
func (h *RobotHandler) HandleWecomGET(c *gin.Context) {
	if !h.config.Robots.Wecom.Enabled {
		c.String(http.StatusNotFound, "")
		return
	}
	// English note.
	echostr := c.Query("echostr")
	msgSignature := c.Query("msg_signature")
	timestamp := c.Query("timestamp")
	nonce := c.Query("nonce")

	// English note.
	signature := h.signWecomRequest(h.config.Robots.Wecom.Token, timestamp, nonce, echostr)
	if signature != msgSignature {
		h.logger.Warn(" URL ", zap.String("expected", msgSignature), zap.String("got", signature))
		c.String(http.StatusBadRequest, "invalid signature")
		return
	}

	if echostr == "" {
		c.String(http.StatusBadRequest, "missing echostr")
		return
	}

	// English note.
	if h.config.Robots.Wecom.EncodingAESKey != "" {
		decrypted, err := wecomDecrypt(h.config.Robots.Wecom.EncodingAESKey, echostr)
		if err != nil {
			h.logger.Warn(" echostr ", zap.Error(err))
			c.String(http.StatusBadRequest, "decrypt failed")
			return
		}
		c.String(http.StatusOK, string(decrypted))
		return
	}

	// English note.
	c.String(http.StatusOK, echostr)
}

// English note.
// English note.
func (h *RobotHandler) signWecomRequest(token, timestamp, nonce, echostr string) string {
	strs := []string{token, timestamp, nonce, echostr}
	sort.Strings(strs)
	s := strings.Join(strs, "")
	hash := sha1.Sum([]byte(s))
	return fmt.Sprintf("%x", hash)
}

// English note.
func wecomDecrypt(encodingAESKey, encryptedB64 string) ([]byte, error) {
	key, err := base64.StdEncoding.DecodeString(encodingAESKey + "=")
	if err != nil {
		return nil, err
	}
	if len(key) != 32 {
		return nil, fmt.Errorf("encoding_aes_key  32 ")
	}
	ciphertext, err := base64.StdEncoding.DecodeString(encryptedB64)
	if err != nil {
		return nil, err
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	iv := key[:16]
	mode := cipher.NewCBCDecrypter(block, iv)
	if len(ciphertext)%aes.BlockSize != 0 {
		return nil, fmt.Errorf("")
	}
	plain := make([]byte, len(ciphertext))
	mode.CryptBlocks(plain, ciphertext)
	// English note.
	n := int(plain[len(plain)-1])
	if n < 1 || n > 32 {
		return nil, fmt.Errorf(" PKCS7 ")
	}
	plain = plain[:len(plain)-n]
	// English note.
	if len(plain) < 20 {
		return nil, fmt.Errorf("")
	}
	msgLen := binary.BigEndian.Uint32(plain[16:20])
	if int(20+msgLen) > len(plain) {
		return nil, fmt.Errorf("")
	}
	return plain[20 : 20+msgLen], nil
}

// English note.
func wecomEncrypt(encodingAESKey, message, corpID string) (string, error) {
	key, err := base64.StdEncoding.DecodeString(encodingAESKey + "=")
	if err != nil {
		return "", err
	}
	if len(key) != 32 {
		return "", fmt.Errorf("encoding_aes_key  32 ")
	}
	// English note.
	random := make([]byte, 16)
	if _, err := rand.Read(random); err != nil {
		// English note.
		for i := range random {
			random[i] = byte(time.Now().UnixNano() % 256)
		}
	}
	msgLen := len(message)
	msgBytes := []byte(message)
	corpBytes := []byte(corpID)
	plain := make([]byte, 16+4+msgLen+len(corpBytes))
	copy(plain[:16], random)
	binary.BigEndian.PutUint32(plain[16:20], uint32(msgLen))
	copy(plain[20:20+msgLen], msgBytes)
	copy(plain[20+msgLen:], corpBytes)
	// English note.
	padding := aes.BlockSize - len(plain)%aes.BlockSize
	pad := bytes.Repeat([]byte{byte(padding)}, padding)
	plain = append(plain, pad...)
	// English note.
	block, err := aes.NewCipher(key)
	if err != nil {
		return "", err
	}
	iv := key[:16]
	ciphertext := make([]byte, len(plain))
	mode := cipher.NewCBCEncrypter(block, iv)
	mode.CryptBlocks(ciphertext, plain)
	return base64.StdEncoding.EncodeToString(ciphertext), nil
}

// English note.
func (h *RobotHandler) HandleWecomPOST(c *gin.Context) {
	if !h.config.Robots.Wecom.Enabled {
		h.logger.Debug("，")
		c.String(http.StatusOK, "")
		return
	}
	// English note.
	timestamp := c.Query("timestamp")
	nonce := c.Query("nonce")
	msgSignature := c.Query("msg_signature")

	// English note.
	bodyRaw, err := io.ReadAll(c.Request.Body)
	if err != nil {
		h.logger.Warn(" POST ", zap.Error(err))
		c.String(http.StatusOK, "")
		return
	}
	h.logger.Debug(" POST ", zap.String("body", string(bodyRaw)))

	// English note.
	// English note.
	token := h.config.Robots.Wecom.Token
	if token != "" {
		if msgSignature == "" {
			h.logger.Warn(" POST ，（ token  msg_signature）")
			c.String(http.StatusOK, "")
			return
		}
		var tmp wecomXML
		if err := xml.Unmarshal(bodyRaw, &tmp); err != nil {
			h.logger.Warn(" POST  XML ", zap.Error(err))
			c.String(http.StatusOK, "")
			return
		}
		expected := h.signWecomRequest(token, timestamp, nonce, tmp.Encrypt)
		if expected != msgSignature {
			h.logger.Warn(" POST ", zap.String("expected", expected), zap.String("got", msgSignature))
			c.String(http.StatusOK, "")
			return
		}
	}

	var body wecomXML
	if err := xml.Unmarshal(bodyRaw, &body); err != nil {
		h.logger.Warn(" POST  XML ", zap.Error(err))
		c.String(http.StatusOK, "")
		return
	}
	h.logger.Debug(" XML ", zap.String("ToUserName", body.ToUserName), zap.String("FromUserName", body.FromUserName), zap.String("MsgType", body.MsgType), zap.String("Content", body.Content), zap.String("Encrypt", body.Encrypt))

	// English note.
	enterpriseID := body.ToUserName

	// English note.
	if body.Encrypt != "" && h.config.Robots.Wecom.EncodingAESKey != "" {
		h.logger.Debug("")
		decrypted, err := wecomDecrypt(h.config.Robots.Wecom.EncodingAESKey, body.Encrypt)
		if err != nil {
			h.logger.Warn("", zap.Error(err))
			c.String(http.StatusOK, "")
			return
		}
		h.logger.Debug("", zap.String("decrypted", string(decrypted)))
		if err := xml.Unmarshal(decrypted, &body); err != nil {
			h.logger.Warn(" XML ", zap.Error(err))
			c.String(http.StatusOK, "")
			return
		}
		h.logger.Debug(" XML ", zap.String("FromUserName", body.FromUserName), zap.String("Content", body.Content))
	}

	userID := body.FromUserName
	text := strings.TrimSpace(body.Content)

	// English note.
	maxReplyLen := 2000
	limitReply := func(s string) string {
		if len(s) > maxReplyLen {
			return s[:maxReplyLen] + "\n\n（，）"
		}
		return s
	}

	if body.MsgType != "text" {
		h.logger.Debug("", zap.String("MsgType", body.MsgType))
		h.sendWecomReply(c, userID, enterpriseID, limitReply("，。"), timestamp, nonce)
		return
	}

	// English note.
	if cmdReply, ok := h.handleRobotCommand("wecom", userID, text); ok {
		h.logger.Debug("，", zap.String("userID", userID), zap.String("text", text))
		h.sendWecomReply(c, userID, enterpriseID, limitReply(cmdReply), timestamp, nonce)
		return
	}

	h.logger.Debug("（ AI）", zap.String("userID", userID), zap.String("text", text))

	// English note.
	// English note.
	c.String(http.StatusOK, "success")

	// English note.
	go func() {
		reply := h.HandleMessage("wecom", userID, text)
		reply = limitReply(reply)
		h.logger.Debug("", zap.String("userID", userID), zap.String("reply", reply))
		// English note.
		h.sendWecomMessageViaAPI(userID, enterpriseID, reply)
	}()
}

// English note.
// English note.
func (h *RobotHandler) sendWecomReply(c *gin.Context, toUser, fromUser, content, timestamp, nonce string) {
	// English note.
	if h.config.Robots.Wecom.EncodingAESKey != "" {
		// English note.
		corpID := h.config.Robots.Wecom.CorpID
		if corpID == "" {
			h.logger.Warn(" CorpID ")
			c.String(http.StatusOK, "")
			return
		}

		// English note.
		plainResp := fmt.Sprintf(`<xml>
<ToUserName><![CDATA[%s]]></ToUserName>
<FromUserName><![CDATA[%s]]></FromUserName>
<CreateTime>%d</CreateTime>
<MsgType><![CDATA[text]]></MsgType>
<Content><![CDATA[%s]]></Content>
</xml>`, toUser, fromUser, time.Now().Unix(), content)

		encrypted, err := wecomEncrypt(h.config.Robots.Wecom.EncodingAESKey, plainResp, corpID)
		if err != nil {
			h.logger.Warn("", zap.Error(err))
			c.String(http.StatusOK, "")
			return
		}
		// English note.
		msgSignature := h.signWecomRequest(h.config.Robots.Wecom.Token, timestamp, nonce, encrypted)

		h.logger.Debug("",
			zap.String("Encrypt", encrypted[:50]+"..."),
			zap.String("MsgSignature", msgSignature),
			zap.String("TimeStamp", timestamp),
			zap.String("Nonce", nonce))

		// English note.
		xmlResp := fmt.Sprintf(`<xml><Encrypt><![CDATA[%s]]></Encrypt><MsgSignature><![CDATA[%s]]></MsgSignature><TimeStamp><![CDATA[%s]]></TimeStamp><Nonce><![CDATA[%s]]></Nonce></xml>`, encrypted, msgSignature, timestamp, nonce)
		// also log the final response body so we can cross-check with the
		// network traffic or developer console
		h.logger.Debug("", zap.String("xml", xmlResp))
		// for additional confidence, decrypt the payload ourselves and log it
		if dec, err2 := wecomDecrypt(h.config.Robots.Wecom.EncodingAESKey, encrypted); err2 == nil {
			h.logger.Debug("", zap.String("plain", string(dec)))
		} else {
			h.logger.Warn("", zap.Error(err2))
		}

		// English note.
		c.Writer.WriteHeader(http.StatusOK)
		// use text/xml as that's what WeCom examples show
		c.Writer.Header().Set("Content-Type", "text/xml; charset=utf-8")
		_, _ = c.Writer.Write([]byte(xmlResp))
		h.logger.Debug("")
		return
	}

	// English note.
	h.logger.Debug("", zap.String("ToUserName", toUser), zap.String("FromUserName", fromUser), zap.String("Content", content[:50]+"..."))

	// English note.
	xmlResp := fmt.Sprintf(`<xml>
<ToUserName><![CDATA[%s]]></ToUserName>
<FromUserName><![CDATA[%s]]></FromUserName>
<CreateTime>%d</CreateTime>
<MsgType><![CDATA[text]]></MsgType>
<Content><![CDATA[%s]]></Content>
</xml>`, toUser, fromUser, time.Now().Unix(), content)

	// log the exact plaintext response for debugging
	h.logger.Debug("", zap.String("xml", xmlResp))

	// use text/xml as recommended by WeCom docs
	c.Header("Content-Type", "text/xml; charset=utf-8")
	c.String(http.StatusOK, xmlResp)
	h.logger.Debug("")
}

// English note.

// English note.
type RobotTestRequest struct {
	Platform string `json:"platform"` //  "dingtalk"、"lark"、"wecom"
	UserID   string `json:"user_id"`
	Text     string `json:"text"`
}

// English note.
func (h *RobotHandler) HandleRobotTest(c *gin.Context) {
	var req RobotTestRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": " JSON， platform、user_id、text"})
		return
	}
	platform := strings.TrimSpace(req.Platform)
	if platform == "" {
		platform = "test"
	}
	userID := strings.TrimSpace(req.UserID)
	if userID == "" {
		userID = "test_user"
	}
	reply := h.HandleMessage(platform, userID, req.Text)
	c.JSON(http.StatusOK, gin.H{"reply": reply})
}

// English note.
func (h *RobotHandler) sendWecomMessageViaAPI(toUser, toParty, content string) {
	if !h.config.Robots.Wecom.Enabled {
		return
	}

	secret := h.config.Robots.Wecom.Secret
	corpID := h.config.Robots.Wecom.CorpID
	agentID := h.config.Robots.Wecom.AgentID

	if secret == "" || corpID == "" {
		h.logger.Warn(" API  secret  corpID ")
		return
	}

	// English note.
	tokenURL := fmt.Sprintf("https://qyapi.weixin.qq.com/cgi-bin/gettoken?corpid=%s&corpsecret=%s", corpID, secret)
	resp, err := http.Get(tokenURL)
	if err != nil {
		h.logger.Warn(" token ", zap.Error(err))
		return
	}
	defer resp.Body.Close()

	var tokenResp struct {
		AccessToken string `json:"access_token"`
		ErrCode     int    `json:"errcode"`
		ErrMsg      string `json:"errmsg"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&tokenResp); err != nil {
		h.logger.Warn(" token ", zap.Error(err))
		return
	}
	if tokenResp.ErrCode != 0 {
		h.logger.Warn(" token ", zap.String("errmsg", tokenResp.ErrMsg), zap.Int("errcode", tokenResp.ErrCode))
		return
	}

	// English note.
	msgReq := map[string]interface{}{
		"touser":  toUser,
		"msgtype": "text",
		"agentid": agentID,
		"text": map[string]interface{}{
			"content": content,
		},
	}

	msgBody, err := json.Marshal(msgReq)
	if err != nil {
		h.logger.Warn("", zap.Error(err))
		return
	}

	// English note.
	sendURL := fmt.Sprintf("https://qyapi.weixin.qq.com/cgi-bin/message/send?access_token=%s", tokenResp.AccessToken)
	msgResp, err := http.Post(sendURL, "application/json", bytes.NewReader(msgBody))
	if err != nil {
		h.logger.Warn("", zap.Error(err))
		return
	}
	defer msgResp.Body.Close()

	var sendResp struct {
		ErrCode     int    `json:"errcode"`
		ErrMsg      string `json:"errmsg"`
		InvalidUser string `json:"invaliduser"`
		MsgID       string `json:"msgid"`
	}
	if err := json.NewDecoder(msgResp.Body).Decode(&sendResp); err != nil {
		h.logger.Warn("", zap.Error(err))
		return
	}

	if sendResp.ErrCode == 0 {
		h.logger.Debug("", zap.String("msgid", sendResp.MsgID))
	} else {
		h.logger.Warn("", zap.String("errmsg", sendResp.ErrMsg), zap.Int("errcode", sendResp.ErrCode), zap.String("invaliduser", sendResp.InvalidUser))
	}
}

// English note.

// English note.
func (h *RobotHandler) HandleDingtalkPOST(c *gin.Context) {
	if !h.config.Robots.Dingtalk.Enabled {
		c.JSON(http.StatusOK, gin.H{})
		return
	}
	// English note.
	c.JSON(http.StatusOK, gin.H{"message": "ok"})
}

// English note.

// English note.
func (h *RobotHandler) HandleLarkPOST(c *gin.Context) {
	if !h.config.Robots.Lark.Enabled {
		c.JSON(http.StatusOK, gin.H{})
		return
	}
	var body struct {
		Challenge string `json:"challenge"`
	}
	if err := c.ShouldBindJSON(&body); err == nil && body.Challenge != "" {
		c.JSON(http.StatusOK, gin.H{"challenge": body.Challenge})
		return
	}
	c.JSON(http.StatusOK, gin.H{})
}
