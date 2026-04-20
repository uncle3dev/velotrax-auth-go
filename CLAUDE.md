# velotrax-auth-go Project Context

## Project Overview
Authentication service for the Velotrax platform. Handles user registration, login, logout, and token refresh using gRPC.

## Tech Stack
- **Language**: Go 1.25
- **Module**: `github.com/uncle3dev/velotrax-auth-go`
- **Framework**: Gin (HTTP), gRPC (service communication)
- **Database**: MongoDB (`velotrax` db, `users` collection)
- **Config**: Viper (reads from environment variables)
- **Logging**: Uber Zap
- **Containerization**: Docker + Docker Compose

## Project Structure
```
cmd/server/main.go              — entrypoint, graceful shutdown
internal/
  config/config.go              — viper config loader
  db/mongo.go                   — MongoDB connect + EnsureIndexes
  model/user.go                 — User model
  middleware/
    cors.go / logger.go / recovery.go
  router/router.go              — gin engine, /health endpoint
Dockerfile                      — multi-stage build (golang:1.25-alpine → alpine:3.21)
docker-compose.yml              — auth service on port 50051
.env                            — local env vars
go.mod / go.sum
```

## Database Schema
User model in `internal/model/user.go`:
```go
type User struct {
    ID           bson.ObjectID   `bson:"_id"                 json:"id"`
    UserName     string          `bson:"userName"            json:"userName"`
    PasswordHash string          `bson:"password"            json:"-"`
    Active       bool            `bson:"active"              json:"active"`
    Roles        []string        `bson:"roles"               json:"roles"`
    OrderIDs     []bson.ObjectID `bson:"order_ids,omitempty" json:"orderIds,omitempty"`
    CreatedAt    time.Time       `bson:"created_at"          json:"createdAt"`
    UpdatedAt    time.Time       `bson:"updated_at"          json:"updatedAt"`
}
```
Available roles: `SHIPPER`, `ADMIN`, `FREE_USER`

## Docker Setup
MongoDB runs in the **gateway stack** (`velotrax-gateway-go`), not here.
Both stacks share the `velotrax-net` Docker network.

Startup order:
```bash
cd velotrax-gateway-go && docker compose up -d   # creates network + MongoDB
cd velotrax-auth-go && docker compose up --build
```

## Configuration (`.env`)
| Key | Value |
|-----|-------|
| `APP_PORT` | `50051` |
| `MONGO_URI` | `mongodb://velotrax-mongodb:27017` |
| `JWT_SECRET` | min 32 chars |
| `JWT_EXPIRY` | `15m` |
| `JWT_REFRESH_EXPIRY` | `168h` |

## gRPC Interface
Service: `auth.AuthService` (proto in `velotrax-gateway-go/proto/auth/auth.proto`)
Generated code: `/Users/uncle3/Projects/velotrax-gateway-go/internal/gen/auth/`

Methods: `Register`, `Login`, `Logout`, `RefreshToken`

## Integration Points
- **Gateway**: `velotrax-gateway-go` (port :8080) calls this service via gRPC
- Gateway pre-configures all API routes pointing to this service's endpoints
