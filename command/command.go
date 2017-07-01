package command

import (
	"regexp"
	"strings"

	"fmt"

	"github.com/meifamily/ptt-alertor/crawler"
	"github.com/meifamily/ptt-alertor/models/ptt/article"
	boardproto "github.com/meifamily/ptt-alertor/models/ptt/board"
	"github.com/meifamily/ptt-alertor/models/subscription"
	"github.com/meifamily/ptt-alertor/models/top"
	user "github.com/meifamily/ptt-alertor/models/user/redis"
	log "github.com/meifamily/logrus"
)

var Commands = map[string]map[string]string{
	"一般": {
		"指令": "可使用的指令清單",
		"清單": "設定的看板、關鍵字、作者",
		"排行": "前五名追蹤的關鍵字、作者",
	},
	"關鍵字相關": {
		"新增 看板 關鍵字": "新增看板關鍵字。",
		"刪除 看板 關鍵字": "刪除看板關鍵字。",
		"範例":        "新增 gossiping,movie 金城武,結衣",
	},
	"作者相關": {
		"新增作者 看板 作者": "新增看板作者。",
		"刪除作者 看板 作者": "刪除看板作者。",
		"範例":         "新增作者 gossiping ffaarr,obov",
	},
	"推文相關": {
		"新增推文 網址": "新增文章推文追蹤",
		"刪除推文 網址": "刪除文章推文追蹤",
		"範例":      "新增推文 https://www.ptt.cc/bbs/EZsoft/M.1497363598.A.74E.html",
	},
}

var commandActionMap = map[string]updateAction{
	"新增":   addKeywords,
	"刪除":   removeKeywords,
	"新增作者": addAuthors,
	"刪除作者": removeAuthors,
	"新增推文": addArticles,
	"刪除推文": removeArticles,
}

func HandleCommand(text string, userID string) string {
	command := strings.Fields(strings.TrimSpace(text))[0]
	log.WithFields(log.Fields{
		"account": userID,
		"command": command,
	}).Info("Command Request")
	switch command {
	case "清單":
		return handleList(userID)
	case "指令":
		return stringCommands()
	case "排行":
		return listTop()
	case "新增", "刪除":
		return handleKeyword(command, userID, text)
	case "新增作者", "刪除作者":
		return handleAuthor(command, userID, text)
	case "新增推文", "刪除推文":
		return handlePush(command, userID, text)
	}
	return "無此指令，請打「指令」查看指令清單"
}

func handleList(userID string) string {
	subs := new(user.User).Find(userID).Subscribes
	if len(subs) == 0 {
		return "尚未建立清單。請打「指令」查看新增方法。"
	}
	return new(user.User).Find(userID).Subscribes.String()
}

func stringCommands() string {
	str := ""
	for cat, cmds := range Commands {
		str += "[" + cat + "]\n"
		for cmd, doc := range cmds {
			str += cmd
			if doc != "" {
				str += "：" + doc
			}
			str += "\n"
		}
		str += "\n"
	}
	return str
}

func listTop() string {
	content := "關鍵字"
	for i, keyword := range top.ListKeywords(5) {
		content += fmt.Sprintf("\n%d. %s", i+1, keyword)
	}
	content += "\n----\n作者"
	for i, author := range top.ListAuthors(5) {
		content += fmt.Sprintf("\n%d. %s", i+1, author)
	}
	return content
}

func handleKeyword(command, userID, text string) string {
	re := regexp.MustCompile("^(新增|刪除)\\s+([^,，][\\w-_,，]*[^,，:\\s]):?\\s+(\\*|.*[^\\s])")
	matched := re.MatchString(text)
	if !matched {
		return inputErrorTips() + "\n\n正確範例：\n" + command + " gossiping,lol 問卦,爆卦"
	}
	args := re.FindStringSubmatch(text)
	boardNames := splitParamString(args[2])
	input := args[3]
	var inputs []string
	if strings.HasPrefix(input, "regexp:") {
		if !checkRegexp(input) {
			return "正規表示式錯誤，請檢查規則。"
		}
		inputs = []string{args[3]}
	} else {
		inputs = splitParamString(args[3])
	}
	log.WithFields(log.Fields{
		"id":      userID,
		"command": command,
		"boards":  boardNames,
		"words":   inputs,
	}).Info("Keyword Command")
	err := update(commandActionMap[command], userID, boardNames, inputs...)
	if msg, ok := checkBoardError(err); ok {
		return msg
	}
	if err != nil {
		return command + "失敗，請嘗試封鎖再解封鎖，並重新執行註冊步驟。\n若問題未解決，請至粉絲團或 LINE 首頁留言。"
	}
	return command + "成功"

}

func handleAuthor(command, userID, text string) string {
	re := regexp.MustCompile("^(新增作者|刪除作者)\\s+([^,，][\\w-_,，]*[^,，:\\s]):?\\s+(\\*|[\\s,\\w]+)")
	matched := re.MatchString(text)
	if !matched {
		return inputErrorTips() + "\n4. 作者為半形英文與數字組成。\n\n正確範例：\n" + command + " gossiping,lol ffaarr,obov"
	}
	args := re.FindStringSubmatch(text)
	boardNames := splitParamString(args[2])
	inputs := splitParamString(args[3])
	log.WithFields(log.Fields{
		"id":      userID,
		"command": command,
		"boards":  boardNames,
		"words":   inputs,
	}).Info("Author Command")
	err := update(commandActionMap[command], userID, boardNames, inputs...)
	if msg, ok := checkBoardError(err); ok {
		return msg
	}
	if err != nil {
		return command + "失敗，請嘗試封鎖再解封鎖，並重新執行註冊步驟。\n若問題未解決，請至粉絲團或 LINE 首頁留言。"
	}
	return command + "成功"
}

func handlePush(command, userID, text string) string {
	re := regexp.MustCompile("^(新增推文|刪除推文)\\s+https?://www.ptt.cc/bbs/([\\w-_]*)/(M\\.\\d+.A.\\w*)\\.html$")
	matched := re.MatchString(text)
	if !matched {
		return "指令格式錯誤。\n1. 網址與指令需至少一個空白。\n2. 網址錯誤格式。\n正確範例：\n" + command + " https://www.ptt.cc/bbs/EZsoft/M.1497363598.A.74E.html"
	}
	args := re.FindStringSubmatch(text)
	boardName := args[2]
	articleCode := args[3]
	log.WithFields(log.Fields{
		"id":      userID,
		"command": command,
		"boards":  boardName,
		"words":   articleCode,
	}).Info("Push Command")
	if !checkArticleExist(boardName, articleCode) {
		return "文章不存在"
	}
	err := update(commandActionMap[command], userID, []string{boardName}, articleCode)
	if err != nil {
		return command + "失敗，請嘗試封鎖再解封鎖，並重新執行註冊步驟。\n若問題未解決，請至粉絲團或 LINE 首頁留言。"
	}
	return command + "成功"
}

func checkArticleExist(boardName, articleCode string) bool {
	a := new(article.Article)
	a.Code = articleCode
	if bl, _ := a.Exist(); bl {
		return true
	}
	if crawler.CheckArticleExist(boardName, articleCode) {
		a.Board = boardName
		initialArticle(*a)
		return true
	}
	return false
}

func initialArticle(a article.Article) error {
	a = crawler.BuildArticle(a.Board, a.Code)
	err := a.Save()
	return err
}

func checkBoardError(err error) (string, bool) {
	if bErr, ok := err.(boardproto.BoardNotExistError); ok {
		return "板名錯誤，請確認拼字。可能板名：\n" + bErr.Suggestion, true
	}
	return "", false
}

func inputErrorTips() string {
	return "指令格式錯誤。\n1. 需以空白分隔動作、板名、關鍵字或作者\n2. 板名欄位開頭與結尾不可有逗號\n3. 板名欄位間不允許空白字元。"
}

func checkRegexp(input string) bool {
	pattern := strings.Replace(strings.TrimPrefix(input, "regexp:"), "//", "////", -1)
	_, err := regexp.Compile(pattern)
	if err != nil {
		return false
	}
	return true
}

func splitParamString(paramString string) (params []string) {
	paramString = strings.Trim(paramString, ",，")
	if !strings.ContainsAny(paramString, ",，") {
		return []string{paramString}
	}

	if strings.Contains(paramString, ",") {
		params = strings.Split(paramString, ",")
	} else {
		params = []string{paramString}
	}

	for i := 0; i < len(params); i++ {
		if strings.Contains(params[i], "，") {
			params = append(params[:i], append(strings.Split(params[i], "，"), params[i+1:]...)...)
			i--
		}
	}

	for i, param := range params {
		params[i] = strings.TrimSpace(param)
	}

	return params
}

func update(action updateAction, account string, boardNames []string, inputs ...string) error {
	u := new(user.User).Find(account)
	if boardNames[0] == "**" {
		boardNames = nil
		for _, uSub := range u.Subscribes {
			boardNames = append(boardNames, uSub.Board)
		}
	}
	for _, boardName := range boardNames {
		sub := subscription.Subscription{
			Board: boardName,
		}
		err := action(&u, sub, inputs...)
		if err != nil {
			return err
		}
		err = u.Update()
		if err != nil {
			log.WithError(err).Error("Subscription Update Error")
			return err
		}
	}
	return nil
}

func HandleLineFollow(id string) error {
	u := new(user.User).Find(id)
	u.Profile.Line = id
	log.WithFields(log.Fields{
		"id":       id,
		"platform": "line",
	}).Info("User Join")
	return handleFollow(u)
}

func HandleMessengerFollow(id string) error {
	u := new(user.User).Find(id)
	u.Profile.Messenger = id
	log.WithFields(log.Fields{
		"id":       id,
		"platform": "messenger",
	}).Info("User Join")
	return handleFollow(u)
}

func handleFollow(u user.User) error {
	if u.Profile.Account != "" {
		u.Enable = true
		u.Update()
	} else {
		if u.Profile.Messenger != "" {
			u.Profile.Account = u.Profile.Messenger
		} else {
			u.Profile.Account = u.Profile.Line
		}
		u.Enable = true
		err := u.Save()
		if err != nil {
			return err
		}
	}
	return nil
}
