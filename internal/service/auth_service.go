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
	err := s.db.Collection(model.CollectionUsers).FindOne(ctx, bson.M{"userName": req.Email}).Decode(&user)
	if err != nil {
		return nil, status.Error(codes.Unauthenticated, "invalid credentials")
	}

	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(req.Password)); err != nil {
		return nil, status.Error(codes.Unauthenticated, "invalid credentials")
	}

	accessToken, err := token.GenerateAccess(user.ID.Hex(), s.cfg)
	if err != nil {
		return nil, status.Error(codes.Internal, "failed to generate token")
	}

	refreshToken, err := token.GenerateRefresh(user.ID.Hex(), s.cfg)
	if err != nil {
		return nil, status.Error(codes.Internal, "failed to generate refresh token")
	}

	response := &authpb.LoginResponse{
		AccessToken:  accessToken,
		RefreshToken: refreshToken,
		ExpiresIn:    int64(s.cfg.JWTExpiry.Seconds()),
		User:         &authpb.UserDetail{Id: user.ID.Hex(), Email: user.Email, Roles: user.Roles},
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
	userID, err := token.ValidateRefresh(req.RefreshToken, s.cfg)
	if err != nil {
		return nil, status.Error(codes.Unauthenticated, "invalid refresh token")
	}

	accessToken, err := token.GenerateAccess(userID, s.cfg)
	if err != nil {
		return nil, status.Error(codes.Internal, "failed to generate token")
	}

	return &authpb.RefreshTokenResponse{
		AccessToken: accessToken,
		ExpiresIn:   int64(s.cfg.JWTExpiry.Seconds()),
	}, nil
}
