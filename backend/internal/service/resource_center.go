package service

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	stdhtml "html"
	"regexp"
	"sort"
	"strings"
	"time"
	"unicode/utf8"

	"entgo.io/ent/dialect/sql"
	dbent "github.com/Wei-Shaw/sub2api/ent"
	"github.com/Wei-Shaw/sub2api/ent/resourcecategory"
	"github.com/Wei-Shaw/sub2api/ent/resourcecomment"
	"github.com/Wei-Shaw/sub2api/ent/resourcelike"
	"github.com/Wei-Shaw/sub2api/ent/resourcenotification"
	"github.com/Wei-Shaw/sub2api/ent/resourcepost"
	"github.com/Wei-Shaw/sub2api/internal/domain"
	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
	xhtml "golang.org/x/net/html"
	"golang.org/x/net/html/atom"
)

const (
	SettingKeyResourceCenterForbidURLs  = "resource_center_forbid_urls"
	SettingKeyResourceCenterBannedWords = "resource_center_banned_words"
	resourcePublished                   = "published"
	resourceDeleted                     = "deleted"
	ResourceTitleMaxLength              = 120
	ResourceContentMaxLength            = 10000
	ResourceCommentMaxLength            = 2000
	ResourceDefaultPageSize             = 10
	ResourceMaxPageSize                 = 50
)

var resourceURLPattern = regexp.MustCompile(`(?i)(?:https?://|www\.)[^\s<]+`)

var (
	ErrResourceCenterDisabled  = infraerrors.NotFound("RESOURCE_CENTER_DISABLED", "resource sharing center is disabled")
	ErrResourceNotFound        = infraerrors.NotFound("RESOURCE_NOT_FOUND", "resource not found")
	ErrResourceContentBlocked  = infraerrors.BadRequest("RESOURCE_CONTENT_BLOCKED", "content contains prohibited words or links")
	ErrResourceTitleRequired   = infraerrors.BadRequest("RESOURCE_TITLE_REQUIRED", "title is required")
	ErrResourceContentRequired = infraerrors.BadRequest("RESOURCE_CONTENT_REQUIRED", "content is required")
	ErrResourceTitleTooLong    = infraerrors.BadRequest("RESOURCE_TITLE_TOO_LONG", "title is too long")
	ErrResourceContentTooLong  = infraerrors.BadRequest("RESOURCE_CONTENT_TOO_LONG", "content is too long")
	ErrResourceNotOwner        = infraerrors.Forbidden("RESOURCE_NOT_OWNER", "you can only delete your own posts and comments")
)

type ResourceCenterConfig struct {
	Enabled     bool     `json:"enabled"`
	ForbidURLs  bool     `json:"forbid_urls"`
	BannedWords []string `json:"banned_words"`
}

type ResourceCategoryView struct {
	ID          int64  `json:"id"`
	Slug        string `json:"slug"`
	Name        string `json:"name"`
	Description string `json:"description"`
}

type ResourceAuthorView struct {
	ID       int64  `json:"id"`
	Username string `json:"username"`
	Role     string `json:"role"`
}

type ResourcePostView struct {
	ID           int64                `json:"id"`
	Category     ResourceCategoryView `json:"category"`
	Author       ResourceAuthorView   `json:"author"`
	Title        string               `json:"title"`
	Content      string               `json:"content"`
	ViewCount    int                  `json:"view_count"`
	LikeCount    int                  `json:"like_count"`
	CommentCount int                  `json:"comment_count"`
	Liked        bool                 `json:"liked"`
	CreatedAt    time.Time            `json:"created_at"`
	UpdatedAt    time.Time            `json:"updated_at"`
	Status       string               `json:"status,omitempty"`
}

type ResourceCommentView struct {
	ID        int64                 `json:"id"`
	PostID    int64                 `json:"post_id"`
	PostTitle string                `json:"post_title,omitempty"`
	ParentID  *int64                `json:"parent_id,omitempty"`
	Author    ResourceAuthorView    `json:"author"`
	Content   string                `json:"content"`
	LikeCount int                   `json:"like_count"`
	Liked     bool                  `json:"liked"`
	CreatedAt time.Time             `json:"created_at"`
	Replies   []ResourceCommentView `json:"replies,omitempty"`
	Status    string                `json:"status,omitempty"`
}

type ResourcePostDetailView struct {
	Post         ResourcePostView      `json:"post"`
	Comments     []ResourceCommentView `json:"comments"`
	CommentsPage ResourcePageInfo      `json:"comments_page"`
}

type ResourcePageInfo struct {
	Page     int  `json:"page"`
	PageSize int  `json:"page_size"`
	Total    int  `json:"total"`
	HasMore  bool `json:"has_more"`
}

type ResourcePostPageView struct {
	Items []ResourcePostView `json:"items"`
	ResourcePageInfo
}

type ResourceCommentPageView struct {
	Items []ResourceCommentView `json:"items"`
	ResourcePageInfo
}

type ResourceNotificationView struct {
	ID        int64              `json:"id"`
	PostID    int64              `json:"post_id"`
	CommentID *int64             `json:"comment_id,omitempty"`
	Kind      string             `json:"kind"`
	Actor     ResourceAuthorView `json:"actor"`
	Read      bool               `json:"read"`
	CreatedAt time.Time          `json:"created_at"`
}

type ResourceCenterService struct {
	client      *dbent.Client
	userRepo    UserRepository
	settingRepo SettingRepository
}

func NewResourceCenterService(client *dbent.Client, userRepo UserRepository, settingRepo SettingRepository) *ResourceCenterService {
	return &ResourceCenterService{client: client, userRepo: userRepo, settingRepo: settingRepo}
}

func (s *ResourceCenterService) Config(ctx context.Context) (ResourceCenterConfig, error) {
	values, err := s.settingRepo.GetMultiple(ctx, []string{SettingKeyResourceCenterEnabled, SettingKeyResourceCenterForbidURLs, SettingKeyResourceCenterBannedWords})
	if err != nil {
		return ResourceCenterConfig{}, err
	}
	cfg := ResourceCenterConfig{Enabled: values[SettingKeyResourceCenterEnabled] != "false", ForbidURLs: values[SettingKeyResourceCenterForbidURLs] == "true", BannedWords: []string{}}
	if raw := strings.TrimSpace(values[SettingKeyResourceCenterBannedWords]); raw != "" {
		_ = json.Unmarshal([]byte(raw), &cfg.BannedWords)
	}
	cfg.BannedWords = normalizeBannedWords(cfg.BannedWords)
	return cfg, nil
}

func (s *ResourceCenterService) PublicConfig(ctx context.Context) (ResourceCenterConfig, error) {
	cfg, err := s.Config(ctx)
	if err != nil {
		return ResourceCenterConfig{}, err
	}
	cfg.BannedWords = []string{}
	return cfg, nil
}

func (s *ResourceCenterService) UpdateConfig(ctx context.Context, cfg ResourceCenterConfig) error {
	cfg.BannedWords = normalizeBannedWords(cfg.BannedWords)
	words, err := json.Marshal(cfg.BannedWords)
	if err != nil {
		return err
	}
	return s.settingRepo.SetMultiple(ctx, map[string]string{
		SettingKeyResourceCenterEnabled:     fmt.Sprintf("%t", cfg.Enabled),
		SettingKeyResourceCenterForbidURLs:  fmt.Sprintf("%t", cfg.ForbidURLs),
		SettingKeyResourceCenterBannedWords: string(words),
	})
}

func (s *ResourceCenterService) ensureEnabled(ctx context.Context) error {
	cfg, err := s.Config(ctx)
	if err != nil {
		return err
	}
	if !cfg.Enabled {
		return ErrResourceCenterDisabled
	}
	return nil
}

func (s *ResourceCenterService) Categories(ctx context.Context, includeDisabled bool) ([]ResourceCategoryView, error) {
	if !includeDisabled {
		if err := s.ensureEnabled(ctx); err != nil {
			return nil, err
		}
	}
	q := s.client.ResourceCategory.Query()
	if !includeDisabled {
		q = q.Where(resourcecategory.EnabledEQ(true))
	}
	items, err := q.Order(resourcecategory.BySortOrder(), resourcecategory.ByID()).All(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]ResourceCategoryView, 0, len(items))
	for _, item := range items {
		out = append(out, categoryView(item))
	}
	return out, nil
}

func (s *ResourceCenterService) ListPosts(ctx context.Context, userID int64, categoryID int64, query, sortBy string, includeDeleted bool) ([]ResourcePostView, error) {
	page, err := s.ListPostsPage(ctx, userID, categoryID, query, sortBy, includeDeleted, 1, ResourceDefaultPageSize)
	if err != nil {
		return nil, err
	}
	return page.Items, nil
}

func (s *ResourceCenterService) ListPostsPage(ctx context.Context, userID int64, categoryID int64, query, sortBy string, includeDeleted bool, page, pageSize int) (*ResourcePostPageView, error) {
	return s.listPostsPage(ctx, userID, categoryID, query, sortBy, includeDeleted, nil, nil, page, pageSize)
}

func (s *ResourceCenterService) ListAdminPostsPage(ctx context.Context, query, sortBy string, start, end *time.Time, page, pageSize int) (*ResourcePostPageView, error) {
	return s.listPostsPage(ctx, 0, 0, query, sortBy, true, start, end, page, pageSize)
}

func (s *ResourceCenterService) listPostsPage(ctx context.Context, userID int64, categoryID int64, query, sortBy string, includeDeleted bool, start, end *time.Time, page, pageSize int) (*ResourcePostPageView, error) {
	if err := s.ensureEnabled(ctx); err != nil && !includeDeleted {
		return nil, err
	}
	page, pageSize = normalizeResourcePage(page, pageSize)
	q := s.client.ResourcePost.Query().Where(resourcepost.StatusEQ(resourcePublished))
	if includeDeleted {
		q = s.client.ResourcePost.Query()
	}
	if categoryID > 0 {
		q = q.Where(resourcepost.CategoryIDEQ(categoryID))
	}
	if query = strings.TrimSpace(query); query != "" {
		q = q.Where(resourcepost.Or(resourcepost.TitleContainsFold(query), resourcepost.ContentContainsFold(query)))
	}
	if start != nil {
		q = q.Where(resourcepost.CreatedAtGTE(*start))
	}
	if end != nil {
		q = q.Where(resourcepost.CreatedAtLT(*end))
	}
	if strings.EqualFold(sortBy, "popular") {
		q = q.Order(resourcepost.ByLikeCount(sql.OrderDesc()), resourcepost.ByCreatedAt(sql.OrderDesc()))
	} else {
		q = q.Order(resourcepost.ByCreatedAt(sql.OrderDesc()))
	}
	total, err := q.Count(ctx)
	if err != nil {
		return nil, err
	}
	items, err := q.Offset((page - 1) * pageSize).Limit(pageSize).All(ctx)
	if err != nil {
		return nil, err
	}
	views, err := s.postViews(ctx, userID, items)
	if err != nil {
		return nil, err
	}
	return &ResourcePostPageView{Items: views, ResourcePageInfo: pageInfo(page, pageSize, total)}, nil
}

func (s *ResourceCenterService) GetPost(ctx context.Context, userID, postID int64) (*ResourcePostDetailView, error) {
	if err := s.ensureEnabled(ctx); err != nil {
		return nil, err
	}
	post, err := s.client.ResourcePost.Query().Where(resourcepost.IDEQ(postID), resourcepost.StatusEQ(resourcePublished)).Only(ctx)
	if err != nil {
		if dbent.IsNotFound(err) {
			return nil, ErrResourceNotFound
		}
		return nil, err
	}
	_ = s.client.ResourcePost.UpdateOneID(postID).AddViewCount(1).Exec(ctx)
	commentsPage, err := s.ListCommentsPage(ctx, userID, postID, 1, ResourceDefaultPageSize, false)
	if err != nil {
		return nil, err
	}
	posts, err := s.postViews(ctx, userID, []*dbent.ResourcePost{post})
	if err != nil {
		return nil, err
	}
	return &ResourcePostDetailView{Post: posts[0], Comments: commentsPage.Items, CommentsPage: commentsPage.ResourcePageInfo}, nil
}

// GetPostForAdmin returns a post and all comments for moderation, including
// soft-deleted records and regardless of the public feature flag.
func (s *ResourceCenterService) GetPostForAdmin(ctx context.Context, postID int64) (*ResourcePostDetailView, error) {
	post, err := s.client.ResourcePost.Query().Where(resourcepost.IDEQ(postID)).Only(ctx)
	if err != nil {
		if dbent.IsNotFound(err) {
			return nil, ErrResourceNotFound
		}
		return nil, err
	}
	commentsPage, err := s.ListCommentsPage(ctx, 0, postID, 1, ResourceDefaultPageSize, true)
	if err != nil {
		return nil, err
	}
	posts, err := s.postViews(ctx, 0, []*dbent.ResourcePost{post})
	if err != nil {
		return nil, err
	}
	return &ResourcePostDetailView{Post: posts[0], Comments: commentsPage.Items, CommentsPage: commentsPage.ResourcePageInfo}, nil
}

func (s *ResourceCenterService) ListCommentsPage(ctx context.Context, userID, postID int64, page, pageSize int, includeDeleted bool) (*ResourceCommentPageView, error) {
	if !includeDeleted {
		if err := s.ensureEnabled(ctx); err != nil {
			return nil, err
		}
	}
	page, pageSize = normalizeResourcePage(page, pageSize)
	q := s.client.ResourceComment.Query().Where(resourcecomment.PostIDEQ(postID), resourcecomment.StatusEQ(resourcePublished))
	if includeDeleted {
		q = s.client.ResourceComment.Query().Where(resourcecomment.PostIDEQ(postID))
	}
	total, err := q.Count(ctx)
	if err != nil {
		return nil, err
	}
	items, err := q.Order(resourcecomment.ByCreatedAt()).Offset((page - 1) * pageSize).Limit(pageSize).All(ctx)
	if err != nil {
		return nil, err
	}
	views, err := s.commentViews(ctx, userID, items)
	if err != nil {
		return nil, err
	}
	return &ResourceCommentPageView{Items: views, ResourcePageInfo: pageInfo(page, pageSize, total)}, nil
}

func (s *ResourceCenterService) ListAdminCommentsPage(ctx context.Context, query string, start, end *time.Time, page, pageSize int) (*ResourceCommentPageView, error) {
	page, pageSize = normalizeResourcePage(page, pageSize)
	q := s.client.ResourceComment.Query()
	if query = strings.TrimSpace(query); query != "" {
		q = q.Where(resourcecomment.ContentContainsFold(query))
	}
	if start != nil {
		q = q.Where(resourcecomment.CreatedAtGTE(*start))
	}
	if end != nil {
		q = q.Where(resourcecomment.CreatedAtLT(*end))
	}
	total, err := q.Count(ctx)
	if err != nil {
		return nil, err
	}
	items, err := q.Order(resourcecomment.ByCreatedAt(sql.OrderDesc())).Offset((page - 1) * pageSize).Limit(pageSize).All(ctx)
	if err != nil {
		return nil, err
	}
	if len(items) == 0 {
		return &ResourceCommentPageView{Items: []ResourceCommentView{}, ResourcePageInfo: pageInfo(page, pageSize, total)}, nil
	}
	views, err := s.commentViews(ctx, 0, items)
	if err != nil {
		return nil, err
	}
	postIDs := make([]int64, 0, len(items))
	for _, item := range items {
		postIDs = append(postIDs, item.PostID)
	}
	posts, err := s.client.ResourcePost.Query().Where(resourcepost.IDIn(postIDs...)).All(ctx)
	if err != nil {
		return nil, err
	}
	titles := make(map[int64]string, len(posts))
	for _, post := range posts {
		titles[post.ID] = post.Title
	}
	for i := range views {
		views[i].PostTitle = titles[views[i].PostID]
	}
	return &ResourceCommentPageView{Items: views, ResourcePageInfo: pageInfo(page, pageSize, total)}, nil
}

func (s *ResourceCenterService) CreatePost(ctx context.Context, userID int64, categoryID int64, title, content string) (*ResourcePostView, error) {
	if err := s.ensureEnabled(ctx); err != nil {
		return nil, err
	}
	title, content = strings.TrimSpace(title), strings.TrimSpace(content)
	if title == "" {
		return nil, ErrResourceTitleRequired
	}
	if utf8.RuneCountInString(title) > ResourceTitleMaxLength {
		return nil, ErrResourceTitleTooLong
	}
	sanitized, plainContent, err := sanitizeResourceContent(content)
	if err != nil {
		return nil, err
	}
	if plainContent == "" {
		return nil, ErrResourceContentRequired
	}
	if utf8.RuneCountInString(plainContent) > ResourceContentMaxLength {
		return nil, ErrResourceContentTooLong
	}
	if err := s.validateContent(ctx, title+"\n"+sanitized); err != nil {
		return nil, err
	}
	cat, err := s.client.ResourceCategory.Query().Where(resourcecategory.IDEQ(categoryID), resourcecategory.EnabledEQ(true)).Only(ctx)
	if err != nil {
		return nil, ErrResourceNotFound
	}
	author, err := s.author(ctx, userID)
	if err != nil {
		return nil, err
	}
	post, err := s.client.ResourcePost.Create().SetCategoryID(cat.ID).SetAuthorID(author.ID).SetAuthorUsername(author.Username).SetAuthorRole(author.Role).SetTitle(title).SetContent(sanitized).Save(ctx)
	if err != nil {
		return nil, err
	}
	views, err := s.postViews(ctx, userID, []*dbent.ResourcePost{post})
	if err != nil {
		return nil, err
	}
	return &views[0], nil
}

func (s *ResourceCenterService) CreateComment(ctx context.Context, userID, postID int64, parentID *int64, content string) (*ResourceCommentView, error) {
	if err := s.ensureEnabled(ctx); err != nil {
		return nil, err
	}
	content = strings.TrimSpace(content)
	sanitized, plainContent, err := sanitizeResourceContent(content)
	if err != nil {
		return nil, err
	}
	if plainContent == "" {
		return nil, ErrResourceContentRequired
	}
	if utf8.RuneCountInString(plainContent) > ResourceCommentMaxLength {
		return nil, ErrResourceContentTooLong
	}
	if err := s.validateContent(ctx, sanitized); err != nil {
		return nil, err
	}
	post, err := s.client.ResourcePost.Query().Where(resourcepost.IDEQ(postID), resourcepost.StatusEQ(resourcePublished)).Only(ctx)
	if err != nil {
		return nil, ErrResourceNotFound
	}
	var replyRecipientID int64
	if parentID != nil {
		parent, err := s.client.ResourceComment.Query().Where(resourcecomment.IDEQ(*parentID), resourcecomment.PostIDEQ(postID), resourcecomment.StatusEQ(resourcePublished)).Only(ctx)
		if err != nil {
			return nil, ErrResourceNotFound
		}
		replyRecipientID = parent.AuthorID
		if parent.ParentID != nil {
			root, err := s.resolveCommentRoot(ctx, postID, parent)
			if err != nil {
				return nil, err
			}
			parentID = &root
		}
	}
	author, err := s.author(ctx, userID)
	if err != nil {
		return nil, err
	}
	b := s.client.ResourceComment.Create().SetPostID(postID).SetAuthorID(author.ID).SetAuthorUsername(author.Username).SetAuthorRole(author.Role).SetContent(sanitized)
	if parentID != nil {
		b.SetParentID(*parentID)
	}
	comment, err := b.Save(ctx)
	if err != nil {
		return nil, err
	}
	_ = s.client.ResourcePost.UpdateOneID(post.ID).AddCommentCount(1).Exec(ctx)
	for _, recipient := range []int64{post.AuthorID} {
		if recipient != userID {
			_ = s.createNotification(ctx, recipient, author, postID, &comment.ID, "comment")
		}
	}
	if parentID != nil && replyRecipientID != userID && replyRecipientID != post.AuthorID {
		_ = s.createNotification(ctx, replyRecipientID, author, postID, &comment.ID, "reply")
	}
	views, err := s.commentViews(ctx, userID, []*dbent.ResourceComment{comment})
	if err != nil {
		return nil, err
	}
	return &views[0], nil
}

func (s *ResourceCenterService) resolveCommentRoot(ctx context.Context, postID int64, comment *dbent.ResourceComment) (int64, error) {
	seen := map[int64]struct{}{}
	for depth := 0; depth < 32; depth++ {
		if _, exists := seen[comment.ID]; exists {
			return 0, ErrResourceNotFound
		}
		seen[comment.ID] = struct{}{}
		if comment.ParentID == nil {
			return comment.ID, nil
		}
		parent, err := s.client.ResourceComment.Query().Where(resourcecomment.IDEQ(*comment.ParentID), resourcecomment.PostIDEQ(postID), resourcecomment.StatusEQ(resourcePublished)).Only(ctx)
		if err != nil {
			return 0, ErrResourceNotFound
		}
		comment = parent
	}
	return 0, ErrResourceNotFound
}

func (s *ResourceCenterService) TogglePostLike(ctx context.Context, userID, postID int64) (bool, error) {
	if err := s.ensureEnabled(ctx); err != nil {
		return false, err
	}
	if _, err := s.client.ResourcePost.Query().Where(resourcepost.IDEQ(postID), resourcepost.StatusEQ(resourcePublished)).Only(ctx); err != nil {
		return false, ErrResourceNotFound
	}
	q := s.client.ResourceLike.Query().Where(resourcelike.UserIDEQ(userID), resourcelike.PostIDEQ(postID))
	if like, err := q.Only(ctx); err == nil {
		_ = s.client.ResourceLike.DeleteOne(like).Exec(ctx)
		_ = s.client.ResourcePost.UpdateOneID(postID).AddLikeCount(-1).Exec(ctx)
		return false, nil
	}
	if err := s.client.ResourceLike.Create().SetUserID(userID).SetPostID(postID).Exec(ctx); err != nil {
		return false, err
	}
	_ = s.client.ResourcePost.UpdateOneID(postID).AddLikeCount(1).Exec(ctx)
	return true, nil
}

func (s *ResourceCenterService) ToggleCommentLike(ctx context.Context, userID, commentID int64) (bool, error) {
	if err := s.ensureEnabled(ctx); err != nil {
		return false, err
	}
	if _, err := s.client.ResourceComment.Query().Where(resourcecomment.IDEQ(commentID), resourcecomment.StatusEQ(resourcePublished)).Only(ctx); err != nil {
		return false, ErrResourceNotFound
	}
	q := s.client.ResourceLike.Query().Where(resourcelike.UserIDEQ(userID), resourcelike.CommentIDEQ(commentID))
	if like, err := q.Only(ctx); err == nil {
		_ = s.client.ResourceLike.DeleteOne(like).Exec(ctx)
		_ = s.client.ResourceComment.UpdateOneID(commentID).AddLikeCount(-1).Exec(ctx)
		return false, nil
	}
	if err := s.client.ResourceLike.Create().SetUserID(userID).SetCommentID(commentID).Exec(ctx); err != nil {
		return false, err
	}
	_ = s.client.ResourceComment.UpdateOneID(commentID).AddLikeCount(1).Exec(ctx)
	return true, nil
}

func (s *ResourceCenterService) Notifications(ctx context.Context, userID int64) ([]ResourceNotificationView, error) {
	items, err := s.client.ResourceNotification.Query().Where(resourcenotification.UserIDEQ(userID)).Order(resourcenotification.ByCreatedAt(sql.OrderDesc())).Limit(100).All(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]ResourceNotificationView, 0, len(items))
	for _, item := range items {
		out = append(out, notificationView(item))
	}
	return out, nil
}

func (s *ResourceCenterService) MarkNotificationRead(ctx context.Context, userID, id int64) error {
	return s.client.ResourceNotification.UpdateOneID(id).Where(resourcenotification.UserIDEQ(userID)).SetRead(true).Exec(ctx)
}

func (s *ResourceCenterService) MarkAllNotificationsRead(ctx context.Context, userID int64) error {
	_, err := s.client.ResourceNotification.Update().Where(resourcenotification.UserIDEQ(userID), resourcenotification.ReadEQ(false)).SetRead(true).Save(ctx)
	return err
}

func (s *ResourceCenterService) DeletePost(ctx context.Context, postID int64) error {
	post, err := s.client.ResourcePost.Query().Where(resourcepost.IDEQ(postID)).Only(ctx)
	if err != nil {
		if dbent.IsNotFound(err) {
			return ErrResourceNotFound
		}
		return err
	}
	return s.deletePost(ctx, post)
}

// DeleteOwnPost deletes a post only when it belongs to the authenticated user.
// The ownership check lives in the service so it cannot be bypassed by hiding
// or tampering with the frontend delete control.
func (s *ResourceCenterService) DeleteOwnPost(ctx context.Context, userID, postID int64) error {
	post, err := s.client.ResourcePost.Query().Where(resourcepost.IDEQ(postID)).Only(ctx)
	if err != nil {
		if dbent.IsNotFound(err) {
			return ErrResourceNotFound
		}
		return err
	}
	if post.AuthorID != userID {
		return ErrResourceNotOwner
	}
	return s.deletePost(ctx, post)
}

func (s *ResourceCenterService) deletePost(ctx context.Context, post *dbent.ResourcePost) error {
	postID := post.ID
	if post.Status == resourceDeleted {
		return nil
	}
	tx, err := s.client.Tx(ctx)
	if err != nil {
		return err
	}
	if err := tx.ResourcePost.UpdateOne(post).SetStatus(resourceDeleted).Exec(ctx); err != nil {
		_ = tx.Rollback()
		return err
	}
	if _, err := tx.ResourceComment.Update().Where(resourcecomment.PostIDEQ(postID), resourcecomment.StatusEQ(resourcePublished)).SetStatus(resourceDeleted).Save(ctx); err != nil {
		_ = tx.Rollback()
		return err
	}
	return tx.Commit()
}

func (s *ResourceCenterService) DeleteComment(ctx context.Context, commentID int64) error {
	comment, err := s.client.ResourceComment.Query().Where(resourcecomment.IDEQ(commentID)).Only(ctx)
	if err != nil {
		if dbent.IsNotFound(err) {
			return ErrResourceNotFound
		}
		return err
	}
	return s.deleteComment(ctx, comment)
}

// DeleteOwnComment deletes a comment only when it belongs to the authenticated user.
func (s *ResourceCenterService) DeleteOwnComment(ctx context.Context, userID, commentID int64) error {
	comment, err := s.client.ResourceComment.Query().Where(resourcecomment.IDEQ(commentID)).Only(ctx)
	if err != nil {
		if dbent.IsNotFound(err) {
			return ErrResourceNotFound
		}
		return err
	}
	if comment.AuthorID != userID {
		return ErrResourceNotOwner
	}
	return s.deleteComment(ctx, comment)
}

func (s *ResourceCenterService) deleteComment(ctx context.Context, comment *dbent.ResourceComment) error {
	if comment.Status == resourceDeleted {
		return nil
	}
	if err := s.client.ResourceComment.UpdateOne(comment).SetStatus(resourceDeleted).Exec(ctx); err != nil {
		return err
	}
	if comment.Status == resourcePublished {
		_, _ = s.client.ResourcePost.UpdateOneID(comment.PostID).AddCommentCount(-1).Save(ctx)
	}
	return nil
}

func (s *ResourceCenterService) BatchDeletePosts(ctx context.Context, ids []int64) (int, error) {
	ids = normalizeResourceIDs(ids)
	if len(ids) == 0 || len(ids) > 100 {
		return 0, ErrResourceNotFound
	}
	tx, err := s.client.Tx(ctx)
	if err != nil {
		return 0, err
	}
	count, err := tx.ResourcePost.Update().Where(resourcepost.IDIn(ids...), resourcepost.StatusNEQ(resourceDeleted)).SetStatus(resourceDeleted).Save(ctx)
	if err != nil {
		_ = tx.Rollback()
		return 0, err
	}
	if _, err := tx.ResourceComment.Update().Where(resourcecomment.PostIDIn(ids...), resourcecomment.StatusEQ(resourcePublished)).SetStatus(resourceDeleted).Save(ctx); err != nil {
		_ = tx.Rollback()
		return 0, err
	}
	if err := tx.Commit(); err != nil {
		return 0, err
	}
	return count, nil
}

func (s *ResourceCenterService) BatchDeleteComments(ctx context.Context, ids []int64) (int, error) {
	ids = normalizeResourceIDs(ids)
	if len(ids) == 0 || len(ids) > 100 {
		return 0, ErrResourceNotFound
	}
	comments, err := s.client.ResourceComment.Query().Where(resourcecomment.IDIn(ids...), resourcecomment.StatusEQ(resourcePublished)).All(ctx)
	if err != nil || len(comments) == 0 {
		return 0, err
	}
	counts := make(map[int64]int)
	for _, comment := range comments {
		counts[comment.PostID]++
	}
	tx, err := s.client.Tx(ctx)
	if err != nil {
		return 0, err
	}
	rollback := func(cause error) (int, error) {
		if rollbackErr := tx.Rollback(); rollbackErr != nil {
			return 0, fmt.Errorf("%w: rollback: %v", cause, rollbackErr)
		}
		return 0, cause
	}
	if _, err := tx.ResourceComment.Update().Where(resourcecomment.IDIn(ids...), resourcecomment.StatusEQ(resourcePublished)).SetStatus(resourceDeleted).Save(ctx); err != nil {
		return rollback(err)
	}
	for postID, count := range counts {
		if _, err := tx.ResourcePost.UpdateOneID(postID).AddCommentCount(-count).Save(ctx); err != nil {
			return rollback(err)
		}
	}
	if err := tx.Commit(); err != nil {
		return 0, err
	}
	return len(comments), nil
}

func normalizeResourceIDs(ids []int64) []int64 {
	seen := make(map[int64]struct{}, len(ids))
	out := make([]int64, 0, len(ids))
	for _, id := range ids {
		if id <= 0 {
			continue
		}
		if _, exists := seen[id]; exists {
			continue
		}
		seen[id] = struct{}{}
		out = append(out, id)
	}
	return out
}

func (s *ResourceCenterService) validateContent(ctx context.Context, content string) error {
	cfg, err := s.Config(ctx)
	if err != nil {
		return err
	}
	if cfg.ForbidURLs && resourceURLPattern.MatchString(content) {
		return ErrResourceContentBlocked
	}
	lower := strings.ToLower(content)
	for _, word := range cfg.BannedWords {
		if word != "" && strings.Contains(lower, strings.ToLower(word)) {
			return ErrResourceContentBlocked
		}
	}
	return nil
}

func normalizeResourcePage(page, pageSize int) (int, int) {
	if page < 1 {
		page = 1
	}
	if pageSize < 1 {
		pageSize = ResourceDefaultPageSize
	}
	if pageSize > ResourceMaxPageSize {
		pageSize = ResourceMaxPageSize
	}
	return page, pageSize
}

func pageInfo(page, pageSize, total int) ResourcePageInfo {
	return ResourcePageInfo{Page: page, PageSize: pageSize, Total: total, HasMore: page*pageSize < total}
}

// sanitizeResourceContent keeps a deliberately small HTML subset. It is used
// on the server even when the browser already sanitized the editor output.
func sanitizeResourceContent(input string) (string, string, error) {
	if len(input) > ResourceContentMaxLength*8 {
		return "", "", ErrResourceContentTooLong
	}
	root := &xhtml.Node{Type: xhtml.ElementNode, Data: "div", DataAtom: atom.Div}
	nodes, err := xhtml.ParseFragment(strings.NewReader(input), root)
	if err != nil {
		return "", "", ErrResourceContentBlocked
	}
	var rendered bytes.Buffer
	var plain strings.Builder
	var render func(*xhtml.Node)
	render = func(node *xhtml.Node) {
		switch node.Type {
		case xhtml.TextNode:
			rendered.WriteString(stdhtml.EscapeString(node.Data))
			plain.WriteString(node.Data)
		case xhtml.ElementNode:
			tag := strings.ToLower(node.Data)
			if tag == "script" || tag == "style" || tag == "iframe" || tag == "object" || tag == "svg" {
				return
			}
			allowed := map[string]bool{"p": true, "br": true, "strong": true, "b": true, "em": true, "i": true, "u": true, "s": true, "ul": true, "ol": true, "li": true, "blockquote": true, "pre": true, "code": true, "a": true}[tag]
			if !allowed {
				for child := node.FirstChild; child != nil; child = child.NextSibling {
					render(child)
				}
				return
			}
			rendered.WriteByte('<')
			rendered.WriteString(tag)
			if tag == "a" {
				for _, attr := range node.Attr {
					if strings.EqualFold(attr.Key, "href") && resourceSafeHref(attr.Val) {
						rendered.WriteString(` href="`)
						rendered.WriteString(stdhtml.EscapeString(attr.Val))
						rendered.WriteByte('"')
						rendered.WriteString(` target="_blank" rel="noopener noreferrer"`)
					}
				}
			}
			rendered.WriteByte('>')
			if tag == "br" {
				plain.WriteByte('\n')
			} else {
				for child := node.FirstChild; child != nil; child = child.NextSibling {
					render(child)
				}
			}
			if tag != "br" {
				rendered.WriteString(`</`)
				rendered.WriteString(tag)
				rendered.WriteByte('>')
			}
		}
	}
	for _, node := range nodes {
		render(node)
	}
	return strings.TrimSpace(rendered.String()), strings.TrimSpace(plain.String()), nil
}

func resourceSafeHref(value string) bool {
	value = strings.TrimSpace(value)
	return strings.HasPrefix(strings.ToLower(value), "https://") || strings.HasPrefix(strings.ToLower(value), "http://")
}

func normalizeBannedWords(words []string) []string {
	seen := map[string]bool{}
	out := make([]string, 0, len(words))
	for _, word := range words {
		word = strings.TrimSpace(word)
		if word != "" && !seen[strings.ToLower(word)] {
			seen[strings.ToLower(word)] = true
			out = append(out, word)
		}
	}
	sort.Strings(out)
	return out
}

func (s *ResourceCenterService) author(ctx context.Context, id int64) (ResourceAuthorView, error) {
	u, err := s.userRepo.GetByID(ctx, id)
	if err != nil || u == nil {
		return ResourceAuthorView{}, errors.New("author not found")
	}
	name := strings.TrimSpace(u.Username)
	if name == "" {
		name = strings.Split(u.Email, "@")[0]
	}
	role := u.Role
	if role != domain.RoleAdmin {
		role = domain.RoleUser
	}
	return ResourceAuthorView{ID: id, Username: name, Role: role}, nil
}
func categoryView(c *dbent.ResourceCategory) ResourceCategoryView {
	return ResourceCategoryView{ID: c.ID, Slug: c.Slug, Name: c.Name, Description: c.Description}
}
func (s *ResourceCenterService) postViews(ctx context.Context, userID int64, items []*dbent.ResourcePost) ([]ResourcePostView, error) {
	if len(items) == 0 {
		return []ResourcePostView{}, nil
	}
	categoryIDs := make([]int64, 0, len(items))
	postIDs := make([]int64, 0, len(items))
	for _, item := range items {
		categoryIDs = append(categoryIDs, item.CategoryID)
		postIDs = append(postIDs, item.ID)
	}
	categories, err := s.client.ResourceCategory.Query().Where(resourcecategory.IDIn(categoryIDs...)).All(ctx)
	if err != nil {
		return nil, err
	}
	categoryByID := make(map[int64]*dbent.ResourceCategory, len(categories))
	for _, category := range categories {
		categoryByID[category.ID] = category
	}
	likedIDs := map[int64]bool{}
	if userID > 0 {
		likes, err := s.client.ResourceLike.Query().Where(resourcelike.UserIDEQ(userID), resourcelike.PostIDIn(postIDs...)).IDs(ctx)
		if err != nil {
			return nil, err
		}
		for _, id := range likes {
			likedIDs[id] = true
		}
	}
	out := make([]ResourcePostView, 0, len(items))
	for _, p := range items {
		cat := categoryByID[p.CategoryID]
		if cat == nil {
			return nil, ErrResourceNotFound
		}
		out = append(out, ResourcePostView{ID: p.ID, Category: categoryView(cat), Author: ResourceAuthorView{ID: p.AuthorID, Username: p.AuthorUsername, Role: p.AuthorRole}, Title: p.Title, Content: p.Content, ViewCount: p.ViewCount, LikeCount: p.LikeCount, CommentCount: p.CommentCount, Liked: likedIDs[p.ID], CreatedAt: p.CreatedAt, UpdatedAt: p.UpdatedAt, Status: p.Status})
	}
	return out, nil
}
func (s *ResourceCenterService) commentViews(ctx context.Context, userID int64, items []*dbent.ResourceComment) ([]ResourceCommentView, error) {
	if len(items) == 0 {
		return []ResourceCommentView{}, nil
	}
	commentIDs := make([]int64, 0, len(items))
	for _, item := range items {
		commentIDs = append(commentIDs, item.ID)
	}
	likedIDs := map[int64]bool{}
	if userID > 0 {
		likes, err := s.client.ResourceLike.Query().Where(resourcelike.UserIDEQ(userID), resourcelike.CommentIDIn(commentIDs...)).IDs(ctx)
		if err != nil {
			return nil, err
		}
		for _, id := range likes {
			likedIDs[id] = true
		}
	}
	out := make([]ResourceCommentView, 0, len(items))
	for _, c := range items {
		out = append(out, ResourceCommentView{ID: c.ID, PostID: c.PostID, ParentID: c.ParentID, Author: ResourceAuthorView{ID: c.AuthorID, Username: c.AuthorUsername, Role: c.AuthorRole}, Content: c.Content, LikeCount: c.LikeCount, Liked: likedIDs[c.ID], CreatedAt: c.CreatedAt, Status: c.Status})
	}
	return out, nil
}
func (s *ResourceCenterService) createNotification(ctx context.Context, userID int64, actor ResourceAuthorView, postID int64, commentID *int64, kind string) error {
	b := s.client.ResourceNotification.Create().SetUserID(userID).SetActorID(actor.ID).SetActorUsername(actor.Username).SetActorRole(actor.Role).SetPostID(postID).SetKind(kind)
	if commentID != nil {
		b.SetCommentID(*commentID)
	}
	return b.Exec(ctx)
}
func notificationView(n *dbent.ResourceNotification) ResourceNotificationView {
	return ResourceNotificationView{ID: n.ID, PostID: n.PostID, CommentID: n.CommentID, Kind: n.Kind, Actor: ResourceAuthorView{ID: n.ActorID, Username: n.ActorUsername, Role: n.ActorRole}, Read: n.Read, CreatedAt: n.CreatedAt}
}
