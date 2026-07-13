package api

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/QSCTech/SRTP-Backend/internal/api/gen"
	"github.com/QSCTech/SRTP-Backend/internal/repository"
	"github.com/QSCTech/SRTP-Backend/internal/service"
	"github.com/QSCTech/SRTP-Backend/models"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"go.uber.org/zap"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestRouterAuthAndProfileFlow(t *testing.T) {
	router, db := newTestRouter(t)

	requestBody := []byte(`{"auth_uid":"auth-1","code":"open-1"}`)
	loginRecorder := httptest.NewRecorder()
	router.ServeHTTP(loginRecorder, jsonRequest(http.MethodPost, "/auth/wx/login", requestBody))
	if loginRecorder.Code != http.StatusOK {
		t.Fatalf("login status=%d body=%s", loginRecorder.Code, loginRecorder.Body.String())
	}
	assertJSONFieldsAbsent(t, loginRecorder.Body.Bytes(), "id")
	var loginResponse gen.UserResponse
	decodeJSON(t, loginRecorder.Body, &loginResponse)
	if _, err := uuid.Parse(loginResponse.PublicId.String()); err != nil {
		t.Fatalf("login public_id=%q is not a valid UUID: %v", loginResponse.PublicId.String(), err)
	}
	if loginResponse.ProfileStatus != "pending" {
		t.Fatalf("login profile_status=%q want pending", loginResponse.ProfileStatus)
	}

	duplicateLoginRecorder := httptest.NewRecorder()
	router.ServeHTTP(duplicateLoginRecorder, jsonRequest(http.MethodPost, "/auth/wx/login", requestBody))
	if duplicateLoginRecorder.Code != http.StatusOK {
		t.Fatalf("duplicate login status=%d body=%s", duplicateLoginRecorder.Code, duplicateLoginRecorder.Body.String())
	}

	var userCount int64
	if err := db.Model(&models.User{}).Where("auth_uid = ?", "auth-1").Count(&userCount).Error; err != nil {
		t.Fatalf("count login user: %v", err)
	}
	if userCount != 1 {
		t.Fatalf("auth-1 user count=%d want 1", userCount)
	}

	var user models.User
	if err := db.Where("auth_uid = ?", "auth-1").First(&user).Error; err != nil {
		t.Fatalf("find login user: %v", err)
	}

	getUserRecorder := httptest.NewRecorder()
	router.ServeHTTP(getUserRecorder, httptest.NewRequest(http.MethodGet, "/users/"+user.PublicID, nil))
	if getUserRecorder.Code != http.StatusOK {
		t.Fatalf("get user status=%d body=%s", getUserRecorder.Code, getUserRecorder.Body.String())
	}

	missingUserRecorder := httptest.NewRecorder()
	router.ServeHTTP(missingUserRecorder, httptest.NewRequest(http.MethodGet, "/users/"+uuid.NewString(), nil))
	if missingUserRecorder.Code != http.StatusNotFound {
		t.Fatalf("missing user status=%d body=%s", missingUserRecorder.Code, missingUserRecorder.Body.String())
	}

	meRecorder := httptest.NewRecorder()
	router.ServeHTTP(meRecorder, httptest.NewRequest(http.MethodGet, "/me", nil))
	if meRecorder.Code != http.StatusUnauthorized {
		t.Fatalf("unauthorized /me status=%d body=%s", meRecorder.Code, meRecorder.Body.String())
	}

	meRecorder = httptest.NewRecorder()
	meRequest := httptest.NewRequest(http.MethodGet, "/me", nil)
	meRequest.Header.Set("X-Auth-UID", "auth-1")
	router.ServeHTTP(meRecorder, meRequest)
	if meRecorder.Code != http.StatusOK {
		t.Fatalf("authorized /me status=%d body=%s", meRecorder.Code, meRecorder.Body.String())
	}

	profileRecorder := httptest.NewRecorder()
	profileRequest := jsonRequest(http.MethodPut, "/me/profile", []byte(`{"nickname":"new nick"}`))
	profileRequest.Header.Set("X-Auth-UID", "auth-1")
	router.ServeHTTP(profileRecorder, profileRequest)
	if profileRecorder.Code != http.StatusOK {
		t.Fatalf("profile status=%d body=%s", profileRecorder.Code, profileRecorder.Body.String())
	}

	var updated models.User
	if err := db.First(&updated, user.ID).Error; err != nil {
		t.Fatalf("find updated user: %v", err)
	}
	if updated.Nickname != "new nick" {
		t.Fatalf("Nickname=%q want new nick", updated.Nickname)
	}
	if updated.ProfileStatus != "pending_review" {
		t.Fatalf("ProfileStatus=%q want pending_review", updated.ProfileStatus)
	}

	var auditCount int64
	if err := db.Model(&models.UserProfileAudit{}).Where("user_id = ?", user.ID).Count(&auditCount).Error; err != nil {
		t.Fatalf("count audits: %v", err)
	}
	if auditCount != 1 {
		t.Fatalf("audit count=%d want 1", auditCount)
	}
}

func TestRouterAllowsUnauthenticatedRoomList(t *testing.T) {
	router, db := newTestRouter(t)
	owner := models.User{AuthUID: "owner-auth", Nickname: "owner"}
	member := models.User{AuthUID: "member-auth", Nickname: "member"}
	if err := db.Create(&owner).Error; err != nil {
		t.Fatalf("create owner: %v", err)
	}
	if err := db.Create(&member).Error; err != nil {
		t.Fatalf("create member: %v", err)
	}
	room := models.Room{
		OwnerID: owner.ID, Name: "public room", SportType: "badminton",
		CampusName: "Zijingang", VenueName: "Gym", Visibility: "public",
		JoinMode: "direct", Status: "recruiting", StartTime: time.Now().Add(time.Hour),
		EndTime: time.Now().Add(2 * time.Hour),
	}
	if err := db.Create(&room).Error; err != nil {
		t.Fatalf("create room: %v", err)
	}
	if err := db.Create(&models.RoomMember{RoomID: room.ID, UserID: member.ID, Role: "member", Status: "joined"}).Error; err != nil {
		t.Fatalf("create room member: %v", err)
	}

	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/rooms", nil))

	if recorder.Code != http.StatusOK {
		t.Fatalf("/rooms status=%d body=%s", recorder.Code, recorder.Body.String())
	}
	var page map[string]any
	decodeJSON(t, recorder.Body, &page)
	items := page["items"].([]any)
	roomCard := items[0].(map[string]any)
	if roomCard["public_id"] != room.PublicID {
		t.Fatalf("room public_id=%v want %s", roomCard["public_id"], room.PublicID)
	}
	if _, exists := roomCard["id"]; exists {
		t.Fatalf("room card leaked internal id: %s", recorder.Body.String())
	}

	detailRecorder := httptest.NewRecorder()
	router.ServeHTTP(detailRecorder, httptest.NewRequest(http.MethodGet, "/rooms/"+room.PublicID, nil))
	if detailRecorder.Code != http.StatusOK {
		t.Fatalf("room detail status=%d body=%s", detailRecorder.Code, detailRecorder.Body.String())
	}
	var detail map[string]any
	decodeJSON(t, detailRecorder.Body, &detail)
	if _, exists := detail["id"]; exists {
		t.Fatalf("room detail leaked internal id: %s", detailRecorder.Body.String())
	}
	ownerResponse := detail["owner"].(map[string]any)
	if _, exists := ownerResponse["id"]; exists {
		t.Fatalf("room owner leaked internal id: %s", detailRecorder.Body.String())
	}
	members := detail["members"].([]any)
	memberResponse := members[0].(map[string]any)
	if _, exists := memberResponse["user_id"]; exists {
		t.Fatalf("room member leaked internal user id: %s", detailRecorder.Body.String())
	}
	if memberResponse["user_public_id"] != member.PublicID {
		t.Fatalf("member public id=%v want %s", memberResponse["user_public_id"], member.PublicID)
	}
}

func TestRouterRoomCreateUpdateCloseFlow(t *testing.T) {
	router, db := newTestRouter(t)
	owner := models.User{AuthUID: "room-owner", Nickname: "owner", ProfileStatus: "approved"}
	other := models.User{AuthUID: "other-user", Nickname: "other", ProfileStatus: "approved"}
	if err := db.Create(&owner).Error; err != nil {
		t.Fatalf("create owner: %v", err)
	}
	if err := db.Create(&other).Error; err != nil {
		t.Fatalf("create other user: %v", err)
	}

	createBody := []byte(`{
		"name":"  羽毛球   约球  ",
		"sport_type":"badminton",
		"campus_name":"紫金港",
		"venue_name":"风雨操场",
		"visibility":"public",
		"join_mode":"direct",
		"start_time":"2030-07-14T09:00:00+08:00",
		"end_time":"2030-07-14T10:00:00+08:00",
		"need_reservation":false,
		"member_limit":1
	}`)
	createRecorder := httptest.NewRecorder()
	createRequest := jsonRequest(http.MethodPost, "/rooms", createBody)
	createRequest.Header.Set("X-Auth-UID", owner.AuthUID)
	router.ServeHTTP(createRecorder, createRequest)
	if createRecorder.Code != http.StatusCreated {
		t.Fatalf("create room status=%d body=%s", createRecorder.Code, createRecorder.Body.String())
	}
	assertJSONFieldsAbsent(t, createRecorder.Body.Bytes(), "id")
	var created gen.RoomDetail
	decodeJSON(t, createRecorder.Body, &created)
	if created.Name != "羽毛球 约球" {
		t.Fatalf("created room name=%q", created.Name)
	}
	if created.Status != "full" {
		t.Fatalf("created room status=%q want full", created.Status)
	}
	if created.InviteCode == nil || len(*created.InviteCode) != 8 {
		t.Fatalf("invite_code=%v want 8 characters", created.InviteCode)
	}
	if created.Owner.PublicId.String() != owner.PublicID {
		t.Fatalf("owner public_id=%s want %s", created.Owner.PublicId, owner.PublicID)
	}
	if len(created.Members) != 1 || created.Members[0].Role != "owner" {
		t.Fatalf("members=%+v want one owner", created.Members)
	}

	var stored models.Room
	if err := db.Where("public_id = ?", created.PublicId.String()).First(&stored).Error; err != nil {
		t.Fatalf("find created room: %v", err)
	}
	var ownerMemberCount int64
	if err := db.Model(&models.RoomMember{}).Where("room_id = ? AND user_id = ? AND role = ? AND status = ?", stored.ID, owner.ID, "owner", "joined").Count(&ownerMemberCount).Error; err != nil {
		t.Fatalf("count owner member: %v", err)
	}
	if ownerMemberCount != 1 {
		t.Fatalf("owner member count=%d want 1", ownerMemberCount)
	}

	updateRecorder := httptest.NewRecorder()
	updateRequest := jsonRequest(http.MethodPut, "/rooms/"+stored.PublicID, []byte(`{"name":"扩容后的房间","member_limit":2}`))
	updateRequest.Header.Set("X-Auth-UID", owner.AuthUID)
	router.ServeHTTP(updateRecorder, updateRequest)
	if updateRecorder.Code != http.StatusOK {
		t.Fatalf("update room status=%d body=%s", updateRecorder.Code, updateRecorder.Body.String())
	}
	var updated gen.RoomDetail
	decodeJSON(t, updateRecorder.Body, &updated)
	if updated.Status != "recruiting" || updated.MemberLimit == nil || *updated.MemberLimit != 2 {
		t.Fatalf("updated room status=%q member_limit=%v", updated.Status, updated.MemberLimit)
	}

	forbiddenRecorder := httptest.NewRecorder()
	forbiddenRequest := jsonRequest(http.MethodPut, "/rooms/"+stored.PublicID, []byte(`{"name":"越权修改"}`))
	forbiddenRequest.Header.Set("X-Auth-UID", other.AuthUID)
	router.ServeHTTP(forbiddenRecorder, forbiddenRequest)
	if forbiddenRecorder.Code != http.StatusBadRequest {
		t.Fatalf("non-owner update status=%d body=%s", forbiddenRecorder.Code, forbiddenRecorder.Body.String())
	}

	createdRoomsRecorder := httptest.NewRecorder()
	createdRoomsRequest := httptest.NewRequest(http.MethodGet, "/me/rooms/created", nil)
	createdRoomsRequest.Header.Set("X-Auth-UID", owner.AuthUID)
	router.ServeHTTP(createdRoomsRecorder, createdRoomsRequest)
	if createdRoomsRecorder.Code != http.StatusOK {
		t.Fatalf("list created rooms status=%d body=%s", createdRoomsRecorder.Code, createdRoomsRecorder.Body.String())
	}
	var createdRooms gen.RoomCardPage
	decodeJSON(t, createdRoomsRecorder.Body, &createdRooms)
	if createdRooms.Total != 1 || len(createdRooms.Items) != 1 {
		t.Fatalf("created rooms total=%d items=%d", createdRooms.Total, len(createdRooms.Items))
	}

	joinedRoomsRecorder := httptest.NewRecorder()
	joinedRoomsRequest := httptest.NewRequest(http.MethodGet, "/me/rooms/joined", nil)
	joinedRoomsRequest.Header.Set("X-Auth-UID", owner.AuthUID)
	router.ServeHTTP(joinedRoomsRecorder, joinedRoomsRequest)
	if joinedRoomsRecorder.Code != http.StatusOK {
		t.Fatalf("list joined rooms status=%d body=%s", joinedRoomsRecorder.Code, joinedRoomsRecorder.Body.String())
	}
	var joinedRooms gen.RoomCardPage
	decodeJSON(t, joinedRoomsRecorder.Body, &joinedRooms)
	if joinedRooms.Total != 0 {
		t.Fatalf("owner joined rooms total=%d want 0", joinedRooms.Total)
	}

	statsRecorder := httptest.NewRecorder()
	statsRequest := httptest.NewRequest(http.MethodGet, "/me/stats", nil)
	statsRequest.Header.Set("X-Auth-UID", owner.AuthUID)
	router.ServeHTTP(statsRecorder, statsRequest)
	if statsRecorder.Code != http.StatusOK {
		t.Fatalf("stats status=%d body=%s", statsRecorder.Code, statsRecorder.Body.String())
	}
	var stats gen.UserStatsResponse
	decodeJSON(t, statsRecorder.Body, &stats)
	if stats.CreatedRoomCount != 1 || stats.JoinedRoomCount != 0 {
		t.Fatalf("stats created=%d joined=%d", stats.CreatedRoomCount, stats.JoinedRoomCount)
	}

	closeRecorder := httptest.NewRecorder()
	closeRequest := httptest.NewRequest(http.MethodPost, "/rooms/"+stored.PublicID+"/close", nil)
	closeRequest.Header.Set("X-Auth-UID", owner.AuthUID)
	router.ServeHTTP(closeRecorder, closeRequest)
	if closeRecorder.Code != http.StatusOK {
		t.Fatalf("close room status=%d body=%s", closeRecorder.Code, closeRecorder.Body.String())
	}
	var closed gen.RoomDetail
	decodeJSON(t, closeRecorder.Body, &closed)
	if closed.Status != "cancelled" {
		t.Fatalf("closed room status=%q want cancelled", closed.Status)
	}
}

func TestRouterRoomMembershipAndApprovalFlow(t *testing.T) {
	router, db := newTestRouter(t)
	owner := models.User{AuthUID: "membership-owner", Nickname: "owner", ProfileStatus: "approved"}
	applicant := models.User{AuthUID: "membership-applicant", Nickname: "applicant", ProfileStatus: "approved"}
	invitee := models.User{AuthUID: "membership-invitee", Nickname: "invitee", ProfileStatus: "approved"}
	for _, user := range []*models.User{&owner, &applicant, &invitee} {
		if err := db.Create(user).Error; err != nil {
			t.Fatalf("create user %s: %v", user.AuthUID, err)
		}
	}
	limit := 2
	approvalRoom := models.Room{
		OwnerID: owner.ID, Name: "approval room", SportType: "badminton",
		CampusName: "Zijingang", VenueName: "Gym", Visibility: "public", JoinMode: "approval",
		Status: "recruiting", ReservationStatus: "not_required", ReservationProvider: "tyys",
		StartTime: time.Now().Add(time.Hour), EndTime: time.Now().Add(2 * time.Hour),
		MemberLimit: &limit, InviteCode: "APPROVE1",
	}
	if err := db.Create(&approvalRoom).Error; err != nil {
		t.Fatalf("create approval room: %v", err)
	}
	now := time.Now()
	if err := db.Create(&models.RoomMember{RoomID: approvalRoom.ID, UserID: owner.ID, Role: "owner", Status: "joined", JoinedAt: &now}).Error; err != nil {
		t.Fatalf("create approval room owner: %v", err)
	}

	directJoinRecorder := httptest.NewRecorder()
	directJoinRequest := httptest.NewRequest(http.MethodPost, "/rooms/"+approvalRoom.PublicID+"/join", nil)
	directJoinRequest.Header.Set("X-Auth-UID", applicant.AuthUID)
	router.ServeHTTP(directJoinRecorder, directJoinRequest)
	if directJoinRecorder.Code != http.StatusBadRequest {
		t.Fatalf("direct join approval room status=%d body=%s", directJoinRecorder.Code, directJoinRecorder.Body.String())
	}

	applyRecorder := httptest.NewRecorder()
	applyRequest := jsonRequest(http.MethodPost, "/rooms/"+approvalRoom.PublicID+"/apply", []byte(`{"message":"想一起打球"}`))
	applyRequest.Header.Set("X-Auth-UID", applicant.AuthUID)
	router.ServeHTTP(applyRecorder, applyRequest)
	if applyRecorder.Code != http.StatusCreated {
		t.Fatalf("apply status=%d body=%s", applyRecorder.Code, applyRecorder.Body.String())
	}
	var applied gen.JoinRequestResponse
	decodeJSON(t, applyRecorder.Body, &applied)
	if applied.Status != "pending" {
		t.Fatalf("apply status=%q want pending", applied.Status)
	}

	duplicateApplyRecorder := httptest.NewRecorder()
	duplicateApplyRequest := jsonRequest(http.MethodPost, "/rooms/"+approvalRoom.PublicID+"/apply", []byte(`{"message":"重复申请"}`))
	duplicateApplyRequest.Header.Set("X-Auth-UID", applicant.AuthUID)
	router.ServeHTTP(duplicateApplyRecorder, duplicateApplyRequest)
	if duplicateApplyRecorder.Code != http.StatusBadRequest {
		t.Fatalf("duplicate apply status=%d body=%s", duplicateApplyRecorder.Code, duplicateApplyRecorder.Body.String())
	}

	unauthorizedListRecorder := httptest.NewRecorder()
	unauthorizedListRequest := httptest.NewRequest(http.MethodGet, "/rooms/"+approvalRoom.PublicID+"/join-requests?status=pending", nil)
	unauthorizedListRequest.Header.Set("X-Auth-UID", applicant.AuthUID)
	router.ServeHTTP(unauthorizedListRecorder, unauthorizedListRequest)
	if unauthorizedListRecorder.Code != http.StatusBadRequest {
		t.Fatalf("non-owner request list status=%d body=%s", unauthorizedListRecorder.Code, unauthorizedListRecorder.Body.String())
	}

	listRecorder := httptest.NewRecorder()
	listRequest := httptest.NewRequest(http.MethodGet, "/rooms/"+approvalRoom.PublicID+"/join-requests?status=pending", nil)
	listRequest.Header.Set("X-Auth-UID", owner.AuthUID)
	router.ServeHTTP(listRecorder, listRequest)
	if listRecorder.Code != http.StatusOK {
		t.Fatalf("list join requests status=%d body=%s", listRecorder.Code, listRecorder.Body.String())
	}
	var requestList gen.JoinRequestListResponse
	decodeJSON(t, listRecorder.Body, &requestList)
	if len(requestList.Items) != 1 || requestList.Items[0].Message != "想一起打球" || requestList.Items[0].UserPublicId.String() != applicant.PublicID {
		t.Fatalf("join request list=%+v", requestList.Items)
	}

	approveRecorder := httptest.NewRecorder()
	approveRequest := jsonRequest(http.MethodPost, "/rooms/"+approvalRoom.PublicID+"/approve", []byte(`{"request_public_id":"`+applied.RequestPublicId.String()+`"}`))
	approveRequest.Header.Set("X-Auth-UID", owner.AuthUID)
	router.ServeHTTP(approveRecorder, approveRequest)
	if approveRecorder.Code != http.StatusOK {
		t.Fatalf("approve status=%d body=%s", approveRecorder.Code, approveRecorder.Body.String())
	}
	var approved gen.JoinRequestResponse
	decodeJSON(t, approveRecorder.Body, &approved)
	if approved.Status != "approved" {
		t.Fatalf("approved status=%q", approved.Status)
	}
	if err := db.First(&approvalRoom, approvalRoom.ID).Error; err != nil {
		t.Fatalf("reload approval room: %v", err)
	}
	if approvalRoom.Status != "full" {
		t.Fatalf("approval room status=%q want full", approvalRoom.Status)
	}

	removeRecorder := httptest.NewRecorder()
	removeRequest := httptest.NewRequest(http.MethodPost, "/rooms/"+approvalRoom.PublicID+"/members/"+applicant.PublicID+"/remove", nil)
	removeRequest.Header.Set("X-Auth-UID", owner.AuthUID)
	router.ServeHTTP(removeRecorder, removeRequest)
	if removeRecorder.Code != http.StatusOK {
		t.Fatalf("remove member status=%d body=%s", removeRecorder.Code, removeRecorder.Body.String())
	}
	var removed gen.MemberActionResponse
	decodeJSON(t, removeRecorder.Body, &removed)
	if removed.Status != "removed" || removed.UserPublicId.String() != applicant.PublicID {
		t.Fatalf("removed response=%+v", removed)
	}
	if err := db.First(&approvalRoom, approvalRoom.ID).Error; err != nil {
		t.Fatalf("reload room after remove: %v", err)
	}
	if approvalRoom.Status != "recruiting" {
		t.Fatalf("room status after remove=%q want recruiting", approvalRoom.Status)
	}

	reapplyRecorder := httptest.NewRecorder()
	reapplyRequest := jsonRequest(http.MethodPost, "/rooms/"+approvalRoom.PublicID+"/apply", []byte(`{"message":"再次申请"}`))
	reapplyRequest.Header.Set("X-Auth-UID", applicant.AuthUID)
	router.ServeHTTP(reapplyRecorder, reapplyRequest)
	if reapplyRecorder.Code != http.StatusCreated {
		t.Fatalf("reapply status=%d body=%s", reapplyRecorder.Code, reapplyRecorder.Body.String())
	}
	var reapplied gen.JoinRequestResponse
	decodeJSON(t, reapplyRecorder.Body, &reapplied)

	rejectRecorder := httptest.NewRecorder()
	rejectRequest := jsonRequest(http.MethodPost, "/rooms/"+approvalRoom.PublicID+"/reject", []byte(`{"request_public_id":"`+reapplied.RequestPublicId.String()+`"}`))
	rejectRequest.Header.Set("X-Auth-UID", owner.AuthUID)
	router.ServeHTTP(rejectRecorder, rejectRequest)
	if rejectRecorder.Code != http.StatusOK {
		t.Fatalf("reject status=%d body=%s", rejectRecorder.Code, rejectRecorder.Body.String())
	}
	var rejected gen.JoinRequestResponse
	decodeJSON(t, rejectRecorder.Body, &rejected)
	if rejected.Status != "rejected" {
		t.Fatalf("rejected status=%q", rejected.Status)
	}

	cancelApplyRecorder := httptest.NewRecorder()
	cancelApplyRequest := jsonRequest(http.MethodPost, "/rooms/"+approvalRoom.PublicID+"/apply", []byte(`{"message":"稍后需要撤销"}`))
	cancelApplyRequest.Header.Set("X-Auth-UID", invitee.AuthUID)
	router.ServeHTTP(cancelApplyRecorder, cancelApplyRequest)
	if cancelApplyRecorder.Code != http.StatusCreated {
		t.Fatalf("cancel test apply status=%d body=%s", cancelApplyRecorder.Code, cancelApplyRecorder.Body.String())
	}
	var cancelApplied gen.JoinRequestResponse
	decodeJSON(t, cancelApplyRecorder.Body, &cancelApplied)

	cancelRecorder := httptest.NewRecorder()
	cancelRequest := httptest.NewRequest(http.MethodPost, "/rooms/"+approvalRoom.PublicID+"/apply/cancel", nil)
	cancelRequest.Header.Set("X-Auth-UID", invitee.AuthUID)
	router.ServeHTTP(cancelRecorder, cancelRequest)
	if cancelRecorder.Code != http.StatusOK {
		t.Fatalf("cancel join request status=%d body=%s", cancelRecorder.Code, cancelRecorder.Body.String())
	}
	var cancelled gen.JoinRequestResponse
	decodeJSON(t, cancelRecorder.Body, &cancelled)
	if cancelled.Status != "cancelled" || cancelled.RequestPublicId != cancelApplied.RequestPublicId {
		t.Fatalf("cancelled response=%+v applied=%+v", cancelled, cancelApplied)
	}

	repeatCancelRecorder := httptest.NewRecorder()
	repeatCancelRequest := httptest.NewRequest(http.MethodPost, "/rooms/"+approvalRoom.PublicID+"/apply/cancel", nil)
	repeatCancelRequest.Header.Set("X-Auth-UID", invitee.AuthUID)
	router.ServeHTTP(repeatCancelRecorder, repeatCancelRequest)
	if repeatCancelRecorder.Code != http.StatusBadRequest {
		t.Fatalf("repeat cancel status=%d body=%s", repeatCancelRecorder.Code, repeatCancelRecorder.Body.String())
	}

	reapplyInviteeRecorder := httptest.NewRecorder()
	reapplyInviteeRequest := jsonRequest(http.MethodPost, "/rooms/"+approvalRoom.PublicID+"/apply", []byte(`{"message":"撤销后重新申请"}`))
	reapplyInviteeRequest.Header.Set("X-Auth-UID", invitee.AuthUID)
	router.ServeHTTP(reapplyInviteeRecorder, reapplyInviteeRequest)
	if reapplyInviteeRecorder.Code != http.StatusCreated {
		t.Fatalf("reapply after cancel status=%d body=%s", reapplyInviteeRecorder.Code, reapplyInviteeRecorder.Body.String())
	}

	inviteRecorder := httptest.NewRecorder()
	inviteRequest := jsonRequest(http.MethodPost, "/rooms/"+approvalRoom.PublicID+"/invite", []byte(`{"user_public_id":"`+invitee.PublicID+`"}`))
	inviteRequest.Header.Set("X-Auth-UID", owner.AuthUID)
	router.ServeHTTP(inviteRecorder, inviteRequest)
	if inviteRecorder.Code != http.StatusOK {
		t.Fatalf("invite status=%d body=%s", inviteRecorder.Code, inviteRecorder.Body.String())
	}

	privateRoom := models.Room{
		OwnerID: owner.ID, Name: "private room", SportType: "tennis",
		CampusName: "Zijingang", VenueName: "Court", Visibility: "private", JoinMode: "direct",
		Status: "recruiting", ReservationStatus: "not_required", ReservationProvider: "tyys",
		StartTime: time.Now().Add(3 * time.Hour), EndTime: time.Now().Add(4 * time.Hour),
		MemberLimit: &limit, InviteCode: "PRIVATE1",
	}
	if err := db.Create(&privateRoom).Error; err != nil {
		t.Fatalf("create private room: %v", err)
	}
	if err := db.Create(&models.RoomMember{RoomID: privateRoom.ID, UserID: owner.ID, Role: "owner", Status: "joined", JoinedAt: &now}).Error; err != nil {
		t.Fatalf("create private room owner: %v", err)
	}
	privateDirectRecorder := httptest.NewRecorder()
	privateDirectRequest := httptest.NewRequest(http.MethodPost, "/rooms/"+privateRoom.PublicID+"/join", nil)
	privateDirectRequest.Header.Set("X-Auth-UID", applicant.AuthUID)
	router.ServeHTTP(privateDirectRecorder, privateDirectRequest)
	if privateDirectRecorder.Code != http.StatusBadRequest {
		t.Fatalf("private direct join status=%d body=%s", privateDirectRecorder.Code, privateDirectRecorder.Body.String())
	}
	codeJoinRecorder := httptest.NewRecorder()
	codeJoinRequest := jsonRequest(http.MethodPost, "/rooms/join-by-code", []byte(`{"invite_code":"private1"}`))
	codeJoinRequest.Header.Set("X-Auth-UID", applicant.AuthUID)
	router.ServeHTTP(codeJoinRecorder, codeJoinRequest)
	if codeJoinRecorder.Code != http.StatusOK {
		t.Fatalf("join by code status=%d body=%s", codeJoinRecorder.Code, codeJoinRecorder.Body.String())
	}
	var codeJoined gen.JoinRoomResult
	decodeJSON(t, codeJoinRecorder.Body, &codeJoined)
	if codeJoined.JoinResult != "joined" || codeJoined.RoomPublicId.String() != privateRoom.PublicID {
		t.Fatalf("join by code response=%+v", codeJoined)
	}

	directRoom := models.Room{
		OwnerID: owner.ID, Name: "direct room", SportType: "basketball",
		CampusName: "Zijingang", VenueName: "Court", Visibility: "public", JoinMode: "direct",
		Status: "recruiting", ReservationStatus: "not_required", ReservationProvider: "tyys",
		StartTime: time.Now().Add(5 * time.Hour), EndTime: time.Now().Add(6 * time.Hour),
		MemberLimit: &limit, InviteCode: "DIRECT01",
	}
	if err := db.Create(&directRoom).Error; err != nil {
		t.Fatalf("create direct room: %v", err)
	}
	if err := db.Create(&models.RoomMember{RoomID: directRoom.ID, UserID: owner.ID, Role: "owner", Status: "joined", JoinedAt: &now}).Error; err != nil {
		t.Fatalf("create direct room owner: %v", err)
	}
	for attempt, wantResult := range []string{"joined", "already_joined"} {
		directRecorder := httptest.NewRecorder()
		directRequest := httptest.NewRequest(http.MethodPost, "/rooms/"+directRoom.PublicID+"/join", nil)
		directRequest.Header.Set("X-Auth-UID", applicant.AuthUID)
		router.ServeHTTP(directRecorder, directRequest)
		if directRecorder.Code != http.StatusOK {
			t.Fatalf("direct join attempt %d status=%d body=%s", attempt, directRecorder.Code, directRecorder.Body.String())
		}
		var result gen.JoinRoomResult
		decodeJSON(t, directRecorder.Body, &result)
		if result.JoinResult != wantResult {
			t.Fatalf("direct join attempt %d result=%q want %q", attempt, result.JoinResult, wantResult)
		}
	}

	leaveRecorder := httptest.NewRecorder()
	leaveRequest := httptest.NewRequest(http.MethodPost, "/rooms/"+directRoom.PublicID+"/leave", nil)
	leaveRequest.Header.Set("X-Auth-UID", applicant.AuthUID)
	router.ServeHTTP(leaveRecorder, leaveRequest)
	if leaveRecorder.Code != http.StatusOK {
		t.Fatalf("leave room status=%d body=%s", leaveRecorder.Code, leaveRecorder.Body.String())
	}
	var left gen.MemberActionResponse
	decodeJSON(t, leaveRecorder.Body, &left)
	if left.Status != "left" || left.UserPublicId.String() != applicant.PublicID {
		t.Fatalf("leave response=%+v", left)
	}
	if err := db.First(&directRoom, directRoom.ID).Error; err != nil {
		t.Fatalf("reload direct room after leave: %v", err)
	}
	if directRoom.Status != "recruiting" {
		t.Fatalf("direct room status after leave=%q want recruiting", directRoom.Status)
	}

	repeatLeaveRecorder := httptest.NewRecorder()
	repeatLeaveRequest := httptest.NewRequest(http.MethodPost, "/rooms/"+directRoom.PublicID+"/leave", nil)
	repeatLeaveRequest.Header.Set("X-Auth-UID", applicant.AuthUID)
	router.ServeHTTP(repeatLeaveRecorder, repeatLeaveRequest)
	if repeatLeaveRecorder.Code != http.StatusBadRequest {
		t.Fatalf("repeat leave status=%d body=%s", repeatLeaveRecorder.Code, repeatLeaveRecorder.Body.String())
	}

	ownerLeaveRecorder := httptest.NewRecorder()
	ownerLeaveRequest := httptest.NewRequest(http.MethodPost, "/rooms/"+directRoom.PublicID+"/leave", nil)
	ownerLeaveRequest.Header.Set("X-Auth-UID", owner.AuthUID)
	router.ServeHTTP(ownerLeaveRecorder, ownerLeaveRequest)
	if ownerLeaveRecorder.Code != http.StatusBadRequest {
		t.Fatalf("owner leave status=%d body=%s", ownerLeaveRecorder.Code, ownerLeaveRecorder.Body.String())
	}

	rejoinRecorder := httptest.NewRecorder()
	rejoinRequest := httptest.NewRequest(http.MethodPost, "/rooms/"+directRoom.PublicID+"/join", nil)
	rejoinRequest.Header.Set("X-Auth-UID", applicant.AuthUID)
	router.ServeHTTP(rejoinRecorder, rejoinRequest)
	if rejoinRecorder.Code != http.StatusOK {
		t.Fatalf("rejoin after leave status=%d body=%s", rejoinRecorder.Code, rejoinRecorder.Body.String())
	}
}

func newTestRouter(t *testing.T) (*gin.Engine, *gorm.DB) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(&models.User{}, &models.UserProfileAudit{}, &models.Room{}, &models.RoomMember{}, &models.JoinRequest{}); err != nil {
		t.Fatalf("auto migrate: %v", err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatalf("get sql db: %v", err)
	}
	userRepo := repository.NewUserRepository(db)
	userService := service.NewUserService(userRepo)
	roomService := service.NewRoomService(repository.NewRoomRepository(db), userService)
	reservationService := service.NewReservationService(repository.NewRoomRepository(db), repository.NewReservationRepository(db))
	return NewRouter(zap.NewNop(), sqlDB, userService, roomService, reservationService), db
}

func jsonRequest(method, target string, body []byte) *http.Request {
	req := httptest.NewRequest(method, target, bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	return req
}

func decodeJSON(t *testing.T, body *bytes.Buffer, target any) {
	t.Helper()
	if err := json.NewDecoder(body).Decode(target); err != nil {
		t.Fatalf("decode response: %v", err)
	}
}

func assertJSONFieldsAbsent(t *testing.T, body []byte, fields ...string) {
	t.Helper()
	var value map[string]any
	if err := json.Unmarshal(body, &value); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	for _, field := range fields {
		if _, exists := value[field]; exists {
			t.Fatalf("response contains forbidden field %q: %s", field, body)
		}
	}
}
