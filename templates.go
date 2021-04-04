package main

import (
	"fmt"
	"html/template"
	"io/fs"
	"log"
	"net/http"
	"time"

	"github.com/gorilla/csrf"
	"github.com/mdbot/wiki/config"
)

type Templates struct {
	fs              fs.FS
	sidebarProvider func() string
}

type CommonArgs struct {
	RequestedUrl   string
	PageTitle      string
	IsWikiPage     bool
	IsError        bool
	CanEdit        bool
	Error          string
	Notice         string
	ShowLinkToView bool
	Sidebar        template.HTML
	User           *config.User
	LastModified   *LastModifiedDetails
	CsrfField      template.HTML
}

type LastModifiedDetails struct {
	User string
	Time time.Time
}

type ViewPageArgs struct {
	Common      CommonArgs
	PageContent template.HTML
}

func (t *Templates) RenderPage(w http.ResponseWriter, r *http.Request, title, content string, log *LastModifiedDetails) {
	t.render("index.gohtml", http.StatusOK, w, &ViewPageArgs{
		Common: t.populateArgs(w, r, CommonArgs{
			PageTitle:    title,
			IsWikiPage:   true,
			LastModified: log,
		}),
		PageContent: template.HTML(content),
	})
}

type EditPageArgs struct {
	Common      CommonArgs
	PageContent string
}

func (t *Templates) RenderEditPage(w http.ResponseWriter, r *http.Request, title, content string) {
	t.render("edit.gohtml", http.StatusOK, w, &EditPageArgs{
		Common: t.populateArgs(w, r, CommonArgs{
			PageTitle: title,
			ShowLinkToView: true,
		}),
		PageContent: content,
	})
}

type DeletePageArgs struct {
	Common CommonArgs
}

func (t *Templates) RenderDeletePage(w http.ResponseWriter, r *http.Request, pageName string) {
	t.render("delete.gohtml", http.StatusOK, w, &DeletePageArgs{
		Common: t.populateArgs(w, r, CommonArgs{
			PageTitle: pageName,
			ShowLinkToView: true,
		}),
	})
}

type RenamePageArgs struct {
	Common CommonArgs
}

func (t *Templates) RenderRenamePage(w http.ResponseWriter, r *http.Request, oldName string) {
	t.render("rename.gohtml", http.StatusOK, w, &RenamePageArgs{
		Common: t.populateArgs(w, r, CommonArgs{
			PageTitle: oldName,
			ShowLinkToView: true,
		}),
	})
}

type ListPagesArgs struct {
	Common CommonArgs
	Pages  []string
}

func (t *Templates) RenderPageList(w http.ResponseWriter, r *http.Request, pages []string) {
	t.render("list.gohtml", http.StatusOK, w, &ListPagesArgs{
		Common: t.populateArgs(w, r, CommonArgs{
			PageTitle: "Pages",
		}),
		Pages: pages,
	})
}

type ListFilesArgs struct {
	Common CommonArgs
	Files  []File
}

func (t *Templates) RenderFileList(w http.ResponseWriter, r *http.Request, files []File) {
	t.render("listfiles.gohtml", http.StatusOK, w, &ListFilesArgs{
		Common: t.populateArgs(w, r, CommonArgs{
			PageTitle: "Files",
		}),
		Files: files,
	})
}

type UploadFileArgs struct {
	Common CommonArgs
}

func (t *Templates) RenderUploadForm(w http.ResponseWriter, r *http.Request) {
	t.render("upload.gohtml", http.StatusOK, w, &UploadFileArgs{
		Common: t.populateArgs(w, r, CommonArgs{
			PageTitle: "Upload file",
		}),
	})
}

type HistoryPageArgs struct {
	Common  CommonArgs
	History []*LogEntry
	Next    string
}

func (t *Templates) RenderHistory(w http.ResponseWriter, r *http.Request, title string, entries []*LogEntry, next string) {
	t.render("history.gohtml", http.StatusOK, w, &HistoryPageArgs{
		Common: t.populateArgs(w, r, CommonArgs{
			PageTitle:  title,
			IsWikiPage: true,
			ShowLinkToView: true,
		}),
		History: entries,
		Next:    next,
	})
}

type RecentChangesArgs struct {
	Common  CommonArgs
	Changes []*RecentChange
	Next    string
}

func (t *Templates) RenderRecentChanges(w http.ResponseWriter, r *http.Request, entries []*RecentChange, next string) {
	t.render("changes.gohtml", http.StatusOK, w, &RecentChangesArgs{
		Common: t.populateArgs(w, r, CommonArgs{
			PageTitle: "Recent changes",
		}),
		Changes: entries,
		Next:    next,
	})
}

type ManageUsersArgs struct {
	Common CommonArgs
	Users  []UserInfo
}

type UserInfo struct {
	Name        string
	Permissions string
}

func (t *Templates) RenderManageUsers(w http.ResponseWriter, r *http.Request, users []UserInfo) {
	t.render("users.gohtml", http.StatusOK, w, &ManageUsersArgs{
		Common: t.populateArgs(w, r, CommonArgs{
			PageTitle: "Manage users",
		}),
		Users: users,
	})
}

type AccountArgs struct {
	Common CommonArgs
}

func (t *Templates) RenderAccount(w http.ResponseWriter, r *http.Request) {
	t.render("account.gohtml", http.StatusOK, w, &AccountArgs{
		Common: t.populateArgs(w, r, CommonArgs{
			PageTitle: "My account",
		}),
	})
}

type ErrorPageArgs struct {
	Common        CommonArgs
	ShowLoginForm bool
	OldPageTitle  string
}

func (t *Templates) RenderNotFound(w http.ResponseWriter, r *http.Request, isWiki bool, pageName string) {
	t.render("404.gohtml", http.StatusNotFound, w, &ErrorPageArgs{
		Common: t.populateArgs(w, r, CommonArgs{
			PageTitle:  "Page not found",
			IsWikiPage: isWiki,
			IsError:    true,
		}),
		OldPageTitle: pageName,
	})
}

func (t *Templates) RenderUnauthorised(w http.ResponseWriter, r *http.Request) {
	t.render("error.gohtml", http.StatusUnauthorized, w, &ErrorPageArgs{
		Common: t.populateArgs(w, r, CommonArgs{
			PageTitle: "Unauthorized",
			IsError:   true,
		}),
		ShowLoginForm: true,
	})
}

func (t *Templates) RenderForbidden(w http.ResponseWriter, r *http.Request) {
	t.render("error.gohtml", http.StatusForbidden, w, &ErrorPageArgs{
		Common: t.populateArgs(w, r, CommonArgs{
			PageTitle: "Forbidden",
			IsError:   true,
		}),
	})
}

func (t *Templates) RenderInternalError(w http.ResponseWriter, r *http.Request) {
	t.render("error.gohtml", http.StatusInternalServerError, w, &ErrorPageArgs{
		Common: t.populateArgs(w, r, CommonArgs{
			PageTitle: "Server Error",
			IsError:   true,
		}),
	})
}

func (t *Templates) RenderBadRequest(w http.ResponseWriter, r *http.Request) {
	t.render("error.gohtml", http.StatusInternalServerError, w, &ErrorPageArgs{
		Common: t.populateArgs(w, r, CommonArgs{
			PageTitle: "Bad Request",
			IsError:   true,
		}),
	})
}

func (t *Templates) render(name string, statusCode int, w http.ResponseWriter, data interface{}) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(statusCode)

	tpl := template.New(name)
	tpl.Funcs(map[string]interface{}{
		"bytes": t.formatBytes,
	})
	template.Must(tpl.ParseFS(t.fs, name, "partials/*.gohtml"))
	if err := tpl.Execute(w, data); err != nil {
		// TODO: We should probably send an error to the client
		log.Printf("Error rendering template: %v\n", err)
	}
}

func (t *Templates) formatBytes(size int64) string {
	const multiple = 1024
	if size < multiple {
		return fmt.Sprintf("%d B", size)
	}

	denominator, power := int64(multiple), 0
	for n := size / multiple; n >= multiple; n /= multiple {
		denominator *= multiple
		power++
	}

	return fmt.Sprintf("%.1f %ciB", float64(size)/float64(denominator), "KMGTPE"[power])
}

func (t *Templates) populateArgs(w http.ResponseWriter, r *http.Request, args CommonArgs) CommonArgs {
	user := getUserForRequest(r)
	args.User = user
	args.CanEdit = user != nil

	if args.Error = getErrorForRequest(r); args.Error != "" {
		clearSessionKey(w, r, sessionErrorKey)
	}

	if args.Notice = getNoticeForRequest(r); args.Notice != "" {
		clearSessionKey(w, r, sessionNoticeKey)
	}

	args.CsrfField = csrf.TemplateField(r)
	args.RequestedUrl = r.URL.String()
	args.Sidebar = template.HTML(t.sidebarProvider())
	return args
}
