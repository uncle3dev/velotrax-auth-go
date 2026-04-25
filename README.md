# velotrax-auth-go

`velotrax-auth-go` là dịch vụ xác thực của Velotrax viết bằng Go. Repo này xử lý đăng ký, đăng nhập, đăng xuất, làm mới access token và đọc/cập nhật profile user qua gRPC; đồng thời có một HTTP server nhỏ cho health check.

## Tech stack

- Go 1.25
- gRPC cho luồng auth chính
- Gin cho HTTP endpoint `/health`
- MongoDB làm nơi lưu user
- JWT cho access token / refresh token
- Viper để đọc cấu hình từ biến môi trường
- Zap cho logging
- Docker / Docker Compose để chạy container

## Cấu trúc chính

```text
cmd/server/main.go        Entrypoint của ứng dụng
internal/config           Load và validate config
internal/db               Kết nối MongoDB + tạo index
internal/model            Model user
internal/service          Logic gRPC auth
internal/token            Phát và validate JWT
internal/router           HTTP router, hiện có `/health`
internal/middleware       Logger, recovery, CORS
internal/interceptor       gRPC unary logger
proto/auth                Proto nguồn
internal/gen/auth         Code sinh từ proto
```

## Luồng chính

- `Register`: tạo user mới, hash mật khẩu bằng bcrypt, gán role mặc định `FREE_USER`.
- `Login`: kiểm tra email/password, trả về `access_token`, `refresh_token`, `expires_in` và thông tin user.
- `RefreshToken`: validate refresh token rồi phát access token mới.
- `GetProfile`: validate access token, đọc user hiện tại theo `sub`, trả về profile.
- `UpdateProfile`: validate access token, cập nhật `user_name` và `roles` của user hiện tại.
- JWT hiện dùng chuẩn `sub` cho user id, có thêm `type` (`access` / `refresh`) và `roles`.
- HTTP server hiện chỉ có `/health` trả về `{ "status": "ok" }`.

## Chạy local

Không có `package.json` hay `Makefile`; dùng lệnh Go trực tiếp.

```bash
go test ./...
go build ./cmd/server
go run ./cmd/server
```

Nếu chạy bằng Docker:

```bash
docker compose up --build
```

Lưu ý:
- `docker-compose.yml` dùng mạng external `velotrax-net`.
- MongoDB không nằm trong repo này; cần một `MONGO_URI` trỏ tới MongoDB đang chạy sẵn.

## Biến môi trường

| Biến | Mặc định | Ghi chú |
|---|---:|---|
| `GRPC_PORT` | `50051` | Cổng gRPC |
| `HTTP_PORT` | `8081` | Cổng HTTP health check |
| `APP_ENV` | `development` | `production` sẽ bật Gin release mode |
| `MONGO_URI` | bắt buộc | URI MongoDB |
| `JWT_SECRET` | `change_me_to_a_strong_secret_at_least_32_chars` | Phải dài ít nhất 32 ký tự |
| `JWT_EXPIRY` | `15m` | TTL của access token |
| `JWT_REFRESH_EXPIRY` | `168h` | TTL của refresh token |
| `ALLOW_ROLE_UPDATE` | `true` | Cho phép `UpdateProfile` sửa `roles` |
| `LOG_LEVEL` | `info` | `debug`, `info`, `warn`, `error` |

## Ghi chú quan trọng

- Khi khởi động, app sẽ connect MongoDB và tạo index cho collection `users`.
- Index hiện có cho collection `users` là `userName` unique và `active` thường.
- File proto nguồn là `proto/auth/auth.proto`; code sinh ra nằm trong `internal/gen/auth` và không nên sửa tay.
- `AuthService` là service gRPC chính với 6 RPC: `Register`, `Login`, `Logout`, `RefreshToken`, `GetProfile`, `UpdateProfile`.
- Token JWT đã chuẩn hóa theo `sub`, `type`, `roles` để downstream service / gateway đọc đồng nhất.
- Nếu chạy bằng `.env` trong repo, nhớ dùng đúng tên biến mà code đọc: `GRPC_PORT`, `HTTP_PORT`, `APP_ENV`, `MONGO_URI`, `JWT_SECRET`, `JWT_EXPIRY`, `JWT_REFRESH_EXPIRY`, `ALLOW_ROLE_UPDATE`.
