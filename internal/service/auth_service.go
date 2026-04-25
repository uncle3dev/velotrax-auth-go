package service

import (
	"context"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.uber.org/zap"
	"golang.org/x/crypto/bcrypt"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/uncle3dev/velotrax-auth-go/internal/config"
	authpb "github.com/uncle3dev/velotrax-auth-go/internal/gen/auth"
	"github.com/uncle3dev/velotrax-auth-go/internal/model"
	"github.com/uncle3dev/velotrax-auth-go/internal/token"
)

type AuthService struct {
	authpb.UnimplementedAuthServiceServer
	db     *mongo.Database
	cfg    *config.Config
	logger *zap.Logger
}

func NewAuthService(db *mongo.Database, cfg *config.Config, logger *zap.Logger) *AuthService {
	return &AuthService{db: db, cfg: cfg, logger: logger}
}

func (s *AuthService) Register(ctx context.Context, req *authpb.RegisterRequest) (*authpb.RegisterResponse, error) {
	hash, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		return nil, status.Error(codes.Internal, "failed to hash password")
	}

	user := model.User{
		ID:           bson.NewObjectID(),
		UserName:     req.FullName,
		Email:        req.Email,
		PasswordHash: string(hash),
		Active:       true,
		Roles:        []string{model.RoleFreeUser},
		CreatedAt:    time.Now(),
		UpdatedAt:    time.Now(),
	}

	_, err = s.db.Collection(model.CollectionUsers).InsertOne(ctx, user)
	if err != nil {
		if mongo.IsDuplicateKeyError(err) {
			return nil, status.Error(codes.AlreadyExists, "email already registered")
		}
		return nil, status.Error(codes.Internal, "failed to create user")
	}

	return &authpb.RegisterResponse{UserId: user.ID.Hex(), Email: req.Email}, nil
}

func (s *AuthService) Login(ctx context.Context, req *authpb.LoginRequest) (*authpb.LoginResponse, error) {
	var user model.User
	err := s.db.Collection(model.CollectionUsers).FindOne(ctx, bson.M{"email": req.Email}).Decode(&user)
	if err != nil {
		return nil, status.Error(codes.Unauthenticated, "invalid credentials")
	}

	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(req.Password)); err != nil {
		return nil, status.Error(codes.Unauthenticated, "invalid credentials")
	}

	accessToken, err := token.GenerateAccess(user.ID.Hex(), user.Roles, s.cfg)
	if err != nil {
		return nil, status.Error(codes.Internal, "failed to generate token")
	}

	refreshToken, err := token.GenerateRefresh(user.ID.Hex(), user.Roles, s.cfg)
	if err != nil {
		return nil, status.Error(codes.Internal, "failed to generate refresh token")
	}

	response := &authpb.LoginResponse{
		AccessToken:  accessToken,
		RefreshToken: refreshToken,
		ExpiresIn:    int64(s.cfg.JWTExpiry.Seconds()),
		User:         userToDetail(user),
	}
	// Log the successful login response for debugging purposes (check for tokens)
	// s.logger.Info("Successful user login",
	// 	zap.String("user_id", user.ID.Hex()),
	// 	zap.String("access_token", accessToken),
	// 	zap.String("refresh_token", refreshToken),
	// )

	return response, nil
}

func (s *AuthService) Logout(ctx context.Context, req *authpb.LogoutRequest) (*authpb.LogoutResponse, error) {
	return &authpb.LogoutResponse{Success: true}, nil
}

func (s *AuthService) RefreshToken(ctx context.Context, req *authpb.RefreshTokenRequest) (*authpb.RefreshTokenResponse, error) {
	userID, _, err := token.ValidateRefresh(req.RefreshToken, s.cfg)
	if err != nil {
		return nil, status.Error(codes.Unauthenticated, "invalid refresh token")
	}

	objectID, err := bson.ObjectIDFromHex(userID)
	if err != nil {
		return nil, status.Error(codes.Unauthenticated, "invalid refresh token")
	}

	var user model.User
	if err := s.db.Collection(model.CollectionUsers).FindOne(ctx, bson.M{"_id": objectID}).Decode(&user); err != nil {
		if err == mongo.ErrNoDocuments {
			return nil, status.Error(codes.NotFound, "user not found")
		}
		return nil, status.Error(codes.Internal, "failed to load user")
	}

	accessToken, err := token.GenerateAccess(userID, user.Roles, s.cfg)
	if err != nil {
		return nil, status.Error(codes.Internal, "failed to generate token")
	}

	return &authpb.RefreshTokenResponse{
		AccessToken: accessToken,
		ExpiresIn:   int64(s.cfg.JWTExpiry.Seconds()),
	}, nil
}

func (s *AuthService) GetProfile(ctx context.Context, req *authpb.GetProfileRequest) (*authpb.GetProfileResponse, error) {
	user, err := s.currentUserFromAccessToken(ctx, req.GetAccessToken())
	if err != nil {
		return nil, err
	}

	return &authpb.GetProfileResponse{User: userToDetail(*user)}, nil
}

func (s *AuthService) UpdateProfile(ctx context.Context, req *authpb.UpdateProfileRequest) (*authpb.UpdateProfileResponse, error) {
	currentUser, err := s.currentUserFromAccessToken(ctx, req.GetAccessToken())
	if err != nil {
		return nil, err
	}

	updates := bson.M{"updated_at": time.Now()}
	changed := false

	if req.GetUserName() != "" && req.GetUserName() != currentUser.UserName {
		updates["userName"] = req.GetUserName()
		changed = true
	}

	if len(req.GetRoles()) > 0 {
		if !s.cfg.AllowRoleUpdate {
			return nil, status.Error(codes.PermissionDenied, "role update is disabled")
		}
		updates["roles"] = req.GetRoles()
		changed = true
	}

	if !changed {
		return nil, status.Error(codes.InvalidArgument, "no profile fields to update")
	}

	if _, err := s.db.Collection(model.CollectionUsers).UpdateByID(ctx, currentUser.ID, bson.M{"$set": updates}); err != nil {
		if mongo.IsDuplicateKeyError(err) {
			return nil, status.Error(codes.AlreadyExists, "user name already taken")
		}
		return nil, status.Error(codes.Internal, "failed to update profile")
	}

	updatedUser, err := s.currentUserByID(ctx, currentUser.ID)
	if err != nil {
		return nil, err
	}

	return &authpb.UpdateProfileResponse{User: userToDetail(*updatedUser)}, nil
}

func (s *AuthService) currentUserFromAccessToken(ctx context.Context, accessToken string) (*model.User, error) {
	if accessToken == "" {
		return nil, status.Error(codes.Unauthenticated, "missing access token")
	}

	userID, _, err := token.ValidateAccess(accessToken, s.cfg)
	if err != nil {
		return nil, status.Error(codes.Unauthenticated, "invalid access token")
	}

	objectID, err := bson.ObjectIDFromHex(userID)
	if err != nil {
		return nil, status.Error(codes.Unauthenticated, "invalid access token")
	}

	return s.currentUserByID(ctx, objectID)
}

func (s *AuthService) currentUserByID(ctx context.Context, id bson.ObjectID) (*model.User, error) {
	var user model.User
	if err := s.db.Collection(model.CollectionUsers).FindOne(ctx, bson.M{"_id": id}).Decode(&user); err != nil {
		if err == mongo.ErrNoDocuments {
			return nil, status.Error(codes.NotFound, "user not found")
		}
		return nil, status.Error(codes.Internal, "failed to load user")
	}
	return &user, nil
}

func userToDetail(user model.User) *authpb.UserDetail {
	return &authpb.UserDetail{
		Id:       user.ID.Hex(),
		Email:    user.Email,
		UserName: user.UserName,
		Roles:    user.Roles,
	}
}
