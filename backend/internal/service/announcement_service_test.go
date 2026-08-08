package service

import (
	"context"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/pagination"
	"github.com/stretchr/testify/require"
)

type announcementRepoStub struct {
	item *Announcement
}

func (s *announcementRepoStub) Create(_ context.Context, a *Announcement) error {
	s.item = a
	return nil
}

func (s *announcementRepoStub) GetByID(_ context.Context, _ int64) (*Announcement, error) {
	if s.item == nil {
		return nil, ErrAnnouncementNotFound
	}
	return s.item, nil
}

func (s *announcementRepoStub) Update(_ context.Context, a *Announcement) error {
	s.item = a
	return nil
}

func (*announcementRepoStub) Delete(context.Context, int64) error {
	return nil
}

func (*announcementRepoStub) List(context.Context, pagination.PaginationParams, AnnouncementListFilters) ([]Announcement, *pagination.PaginationResult, error) {
	return nil, nil, nil
}

func (*announcementRepoStub) ListActive(context.Context, time.Time) ([]Announcement, error) {
	return nil, nil
}

type announcementUserRepoStub struct {
	users []User
}

func (s *announcementUserRepoStub) Create(context.Context, *User) error { return nil }
func (s *announcementUserRepoStub) CreateWithEmailAliasGuard(ctx context.Context, user *User) error {
	return s.Create(ctx, user)
}
func (s *announcementUserRepoStub) GetByID(_ context.Context, id int64) (*User, error) {
	for i := range s.users {
		if s.users[i].ID == id {
			return &s.users[i], nil
		}
	}
	return nil, ErrUserNotFound
}
func (s *announcementUserRepoStub) GetByIDIncludeDeleted(ctx context.Context, id int64) (*User, error) {
	return s.GetByID(ctx, id)
}
func (s *announcementUserRepoStub) GetByEmail(context.Context, string) (*User, error) {
	return nil, ErrUserNotFound
}
func (s *announcementUserRepoStub) GetFirstAdmin(context.Context) (*User, error) {
	return nil, ErrUserNotFound
}
func (s *announcementUserRepoStub) Update(context.Context, *User) error { return nil }
func (s *announcementUserRepoStub) Delete(context.Context, int64) error { return nil }
func (s *announcementUserRepoStub) GetUserAvatar(context.Context, int64) (*UserAvatar, error) {
	return nil, nil
}
func (s *announcementUserRepoStub) UpsertUserAvatar(context.Context, int64, UpsertUserAvatarInput) (*UserAvatar, error) {
	return nil, nil
}
func (s *announcementUserRepoStub) DeleteUserAvatar(context.Context, int64) error { return nil }
func (s *announcementUserRepoStub) List(_ context.Context, params pagination.PaginationParams) ([]User, *pagination.PaginationResult, error) {
	return s.ListWithFilters(context.Background(), params, UserListFilters{})
}
func (s *announcementUserRepoStub) ListWithFilters(_ context.Context, params pagination.PaginationParams, filters UserListFilters) ([]User, *pagination.PaginationResult, error) {
	filtered := make([]User, 0, len(s.users))
	for i := range s.users {
		if filters.Status != "" && s.users[i].Status != filters.Status {
			continue
		}
		filtered = append(filtered, s.users[i])
	}
	if params.Page <= 0 {
		params.Page = 1
	}
	if params.PageSize <= 0 {
		params.PageSize = len(filtered)
	}
	start := (params.Page - 1) * params.PageSize
	if start > len(filtered) {
		start = len(filtered)
	}
	end := start + params.PageSize
	if end > len(filtered) {
		end = len(filtered)
	}
	pages := 0
	if params.PageSize > 0 {
		pages = (len(filtered) + params.PageSize - 1) / params.PageSize
	}
	return filtered[start:end], &pagination.PaginationResult{
		Page:     params.Page,
		PageSize: params.PageSize,
		Total:    int64(len(filtered)),
		Pages:    pages,
	}, nil
}
func (s *announcementUserRepoStub) GetLatestUsedAtByUserIDs(context.Context, []int64) (map[int64]*time.Time, error) {
	return nil, nil
}
func (s *announcementUserRepoStub) GetLatestUsedAtByUserID(context.Context, int64) (*time.Time, error) {
	return nil, nil
}
func (s *announcementUserRepoStub) UpdateUserLastActiveAt(context.Context, int64, time.Time) error {
	return nil
}
func (s *announcementUserRepoStub) UpdateBalance(context.Context, int64, float64) error { return nil }
func (s *announcementUserRepoStub) DeductBalance(context.Context, int64, float64) error { return nil }
func (s *announcementUserRepoStub) UpdateConcurrency(context.Context, int64, int) error { return nil }
func (s *announcementUserRepoStub) BatchSetConcurrency(context.Context, []int64, int) (int, error) {
	return 0, nil
}
func (s *announcementUserRepoStub) BatchAddConcurrency(context.Context, []int64, int) (int, error) {
	return 0, nil
}
func (s *announcementUserRepoStub) BatchUpdateLimits(context.Context, []int64, *int, *int) (int, error) {
	return 0, nil
}
func (s *announcementUserRepoStub) ExistsByEmail(context.Context, string) (bool, error) {
	return false, nil
}
func (s *announcementUserRepoStub) ExistsByEmailAlias(context.Context, string) (bool, error) {
	return false, nil
}
func (s *announcementUserRepoStub) RemoveGroupFromAllowedGroups(context.Context, int64) (int64, error) {
	return 0, nil
}
func (s *announcementUserRepoStub) AddGroupToAllowedGroups(context.Context, int64, int64) error {
	return nil
}
func (s *announcementUserRepoStub) RemoveGroupFromUserAllowedGroups(context.Context, int64, int64) error {
	return nil
}
func (s *announcementUserRepoStub) ListUserAuthIdentities(context.Context, int64) ([]UserAuthIdentityRecord, error) {
	return nil, nil
}
func (s *announcementUserRepoStub) UnbindUserAuthProvider(context.Context, int64, string) error {
	return nil
}
func (s *announcementUserRepoStub) UpdateTotpSecret(context.Context, int64, *string) error {
	return nil
}
func (s *announcementUserRepoStub) EnableTotp(context.Context, int64) error  { return nil }
func (s *announcementUserRepoStub) DisableTotp(context.Context, int64) error { return nil }

func newAnnouncementEmailPushService(t *testing.T, users []User) (*AnnouncementService, *notificationEmailTestSMTPServer) {
	t.Helper()
	smtpServer := startNotificationEmailTestSMTPServer(t)
	settings := newNotificationEmailMemorySettingRepo()
	for key, value := range smtpServer.settings() {
		require.NoError(t, settings.Set(context.Background(), key, value))
	}
	require.NoError(t, settings.Set(context.Background(), SettingKeySiteName, "51token"))
	require.NoError(t, settings.Set(context.Background(), SettingKeyAPIBaseURL, "https://example.com/51Token/v1"))

	emailSvc := NewEmailService(settings, nil)
	notificationSvc := NewNotificationEmailService(settings, emailSvc)
	announcementRepo := &announcementRepoStub{}
	svc := NewAnnouncementService(announcementRepo, nil, &announcementUserRepoStub{users: users}, nil)
	svc.SetNotificationEmailService(notificationSvc)
	return svc, smtpServer
}

func TestAnnouncementServiceCreateRejectsEqualStartEndTimes(t *testing.T) {
	repo := &announcementRepoStub{}
	svc := NewAnnouncementService(repo, nil, nil, nil)
	now := time.Unix(1776790020, 0)

	_, err := svc.Create(context.Background(), &CreateAnnouncementInput{
		Title:      "公告",
		Content:    "内容",
		Status:     AnnouncementStatusActive,
		NotifyMode: AnnouncementNotifyModePopup,
		StartsAt:   &now,
		EndsAt:     &now,
	})
	require.ErrorIs(t, err, ErrAnnouncementInvalidSchedule)
}

func TestAnnouncementServiceCreateEmailPushNoneDoesNotSend(t *testing.T) {
	svc, smtpServer := newAnnouncementEmailPushService(t, []User{
		{ID: 1, Email: "user@example.com", Username: "user", Status: StatusActive},
	})

	_, err := svc.Create(context.Background(), &CreateAnnouncementInput{
		Title:         "公告",
		Content:       "内容",
		Status:        AnnouncementStatusActive,
		NotifyMode:    AnnouncementNotifyModePopup,
		EmailPushMode: AnnouncementEmailPushModeNone,
	})

	require.NoError(t, err)
	require.EqualValues(t, 0, smtpServer.messageCount())
}

func TestAnnouncementServiceCreateSelectedEmailPushSendsSelectedActiveUsers(t *testing.T) {
	svc, smtpServer := newAnnouncementEmailPushService(t, []User{
		{ID: 1, Email: "one@example.com", Username: "one", Status: StatusActive},
		{ID: 2, Email: "two@example.com", Username: "two", Status: StatusDisabled},
		{ID: 3, Email: "three@example.com", Username: "three", Status: StatusActive},
	})

	_, err := svc.Create(context.Background(), &CreateAnnouncementInput{
		Title:            "公告",
		Content:          "内容",
		Status:           AnnouncementStatusActive,
		NotifyMode:       AnnouncementNotifyModePopup,
		EmailPushMode:    AnnouncementEmailPushModeSelected,
		EmailPushUserIDs: []int64{1, 2, 1},
	})

	require.NoError(t, err)
	require.EqualValues(t, 1, smtpServer.messageCount())
}

func TestAnnouncementServiceCreateAllEmailPushSendsAllActiveUsers(t *testing.T) {
	svc, smtpServer := newAnnouncementEmailPushService(t, []User{
		{ID: 1, Email: "one@example.com", Username: "one", Status: StatusActive},
		{ID: 2, Email: "two@example.com", Username: "two", Status: StatusDisabled},
		{ID: 3, Email: "three@example.com", Username: "three", Status: StatusActive},
	})

	_, err := svc.Create(context.Background(), &CreateAnnouncementInput{
		Title:         "公告",
		Content:       "内容",
		Status:        AnnouncementStatusActive,
		NotifyMode:    AnnouncementNotifyModePopup,
		EmailPushMode: AnnouncementEmailPushModeAll,
	})

	require.NoError(t, err)
	require.EqualValues(t, 2, smtpServer.messageCount())
}

func TestAnnouncementServiceUpdateRejectsEqualStartEndTimes(t *testing.T) {
	repo := &announcementRepoStub{
		item: &Announcement{
			ID:         1,
			Title:      "公告",
			Content:    "内容",
			Status:     AnnouncementStatusActive,
			NotifyMode: AnnouncementNotifyModePopup,
		},
	}
	svc := NewAnnouncementService(repo, nil, nil, nil)
	now := time.Unix(1776790020, 0)
	startsAt := &now
	endsAt := &now

	_, err := svc.Update(context.Background(), 1, &UpdateAnnouncementInput{
		StartsAt: &startsAt,
		EndsAt:   &endsAt,
	})
	require.ErrorIs(t, err, ErrAnnouncementInvalidSchedule)
}
