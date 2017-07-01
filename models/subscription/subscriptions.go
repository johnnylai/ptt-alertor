package subscription

import (
	"sort"
	"strings"

	boardProto "github.com/meifamily/ptt-alertor/models/ptt/board"
	board "github.com/meifamily/ptt-alertor/models/ptt/board/redis"
)

type Subscriptions []Subscription

func (ss Subscriptions) String() string {

	sort.Slice(ss, func(i, j int) bool {
		return ss[i].Board < ss[j].Board
	})

	str := "關鍵字\n"
	for _, sub := range ss {
		if sub.String() != "" {
			str += sub.String() + "\n"
		}
	}
	str += "----\n作者\n"
	for _, sub := range ss {
		if sub.StringAuthor() != "" {
			str += sub.StringAuthor() + "\n"
		}
	}
	str += "----\n推文\n"
	for _, sub := range ss {
		if sub.StringArticle() != "" {
			str += sub.StringArticle() + "\n"
		}
	}

	return str
}

func (ss *Subscriptions) Add(sub Subscription) error {
	sub.Board = strings.ToLower(sub.Board)
	if ok, suggestion := board.CheckBoardExist(sub.Board); !ok {
		return boardProto.BoardNotExistError{suggestion}
	}
	sub.CleanUp()
	for i, s := range *ss {
		if strings.EqualFold(s.Board, sub.Board) {
			s.Keywords.AppendNonRepeat(sub.Keywords, false)
			s.Authors.AppendNonRepeat(sub.Authors, false)
			s.Articles.AppendNonRepeat(sub.Articles, false)
			(*ss)[i] = s
			return nil
		}
	}
	*ss = append(*ss, sub)

	return nil
}

func (ss *Subscriptions) Remove(sub Subscription) error {
	sub.Board = strings.ToLower(sub.Board)
	if ok, suggestion := board.CheckBoardExist(sub.Board); !ok {
		return boardProto.BoardNotExistError{suggestion}
	}
	sub.CleanUp()
	for i := 0; i < len(*ss); i++ {
		s := (*ss)[i]
		if strings.EqualFold(s.Board, sub.Board) {
			s.DeleteKeywords(sub.Keywords)
			s.DeleteAuthors(sub.Authors)
			s.DeleteArticles(sub.Articles)
			(*ss)[i] = s
		}
		if isSubEmpty((*ss)[i]) {
			*ss = append((*ss)[:i], (*ss)[i+1:]...)
			i--
		}
	}
	return nil
}

func isSubEmpty(sub Subscription) bool {
	return len(sub.Keywords) == 0 && len(sub.Authors) == 0 && len(sub.Articles) == 0
}
