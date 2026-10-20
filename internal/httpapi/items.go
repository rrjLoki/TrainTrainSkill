package httpapi

import (
	"errors"
	"net/http"
	"strings"
	"unicode/utf8"

	"github.com/rrjLoki/TrainTrainSkill/internal/store"
)

const maxTitleRunes = 200

type createItemRequest struct {
	Title string `json:"title"`
}

type updateItemRequest struct {
	Title *string `json:"title"`
	Done  *bool   `json:"done"`
}

func (a *api) listItems(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{"items": a.store.List()})
}

func (a *api) createItem(w http.ResponseWriter, r *http.Request) {
	var req createItemRequest
	if err := decodeJSON(w, r, &req); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	title := strings.TrimSpace(req.Title)
	if title == "" {
		writeError(w, http.StatusBadRequest, "title 不能为空")
		return
	}
	if utf8.RuneCountInString(title) > maxTitleRunes {
		writeError(w, http.StatusBadRequest, "title 太长")
		return
	}

	writeJSON(w, http.StatusCreated, a.store.Create(title))
}

func (a *api) getItem(w http.ResponseWriter, r *http.Request) {
	it, err := a.store.Get(r.PathValue("id"))
	if err != nil {
		a.itemError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, it)
}

func (a *api) updateItem(w http.ResponseWriter, r *http.Request) {
	var req updateItemRequest
	if err := decodeJSON(w, r, &req); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	if req.Title == nil && req.Done == nil {
		writeError(w, http.StatusBadRequest, "至少提供一个可更新字段")
		return
	}
	if req.Title != nil {
		trimmed := strings.TrimSpace(*req.Title)
		if trimmed == "" {
			writeError(w, http.StatusBadRequest, "title 不能为空")
			return
		}
		if utf8.RuneCountInString(trimmed) > maxTitleRunes {
			writeError(w, http.StatusBadRequest, "title 太长")
			return
		}
		req.Title = &trimmed
	}

	it, err := a.store.Update(r.PathValue("id"), req.Title, req.Done)
	if err != nil {
		a.itemError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, it)
}

func (a *api) deleteItem(w http.ResponseWriter, r *http.Request) {
	if err := a.store.Delete(r.PathValue("id")); err != nil {
		a.itemError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (a *api) itemError(w http.ResponseWriter, err error) {
	if errors.Is(err, store.ErrNotFound) {
		writeError(w, http.StatusNotFound, "记录不存在")
		return
	}
	a.logger.Error("store error", "err", err)
	writeError(w, http.StatusInternalServerError, "服务内部错误")
}
