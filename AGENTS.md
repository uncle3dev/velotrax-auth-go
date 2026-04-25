# AGENTS.md

Tài liệu ngắn cho người/agent tiếp tục làm việc trong repo này.

## Quy ước làm việc

- Ưu tiên đọc file thật trước khi sửa: `cmd/server/main.go`, `internal/config/config.go`, `internal/service/auth_service.go`, `internal/token/jwt.go`, `internal/db/mongo.go`, `proto/auth/auth.proto`.
- Nếu feature chạm config, nhớ đọc thêm `internal/config/config.go` và kiểm tra tên biến môi trường thực tế trước khi đổi docs.
- Không sửa tay file generated trong `internal/gen/auth`.
- Giữ thay đổi hẹp, đúng module đang chịu trách nhiệm.
- Dùng `apply_patch` khi cần chỉnh file.
- Không tự thêm feature ngoài những gì codebase đang có.

## Source of truth

- Entrypoint: `cmd/server/main.go`
- Config: `internal/config/config.go`
- MongoDB: `internal/db/mongo.go`
- Auth business logic: `internal/service/auth_service.go`
- JWT: `internal/token/jwt.go`
- API contract: `proto/auth/auth.proto`
- HTTP routes: `internal/router/router.go`

## Khi sửa auth / token

- Luôn giữ JWT nhất quán giữa access và refresh; token hiện cần có `sub` cho user id, `type` là `access` hoặc `refresh`, và `roles` nếu có.
- Nếu đổi payload JWT, kiểm tra cả login và refresh flow.
- Nếu downstream/gateway parse claim theo `sub`, không quay lại dùng `user_id` làm field chính.
- Profile user hiện có 2 RPC: `GetProfile` và `UpdateProfile`.
- `UpdateProfile` cho phép sửa `user_name` và `roles`; flag `ALLOW_ROLE_UPDATE` dùng để tắt/mở sửa `roles` mà không đổi API.

## Khi sửa route / HTTP

- HTTP server hiện chỉ có `/health`.
- `internal/router/router.go` là nơi gắn route HTTP.
- Middleware HTTP gồm logger, recovery và CORS.

## Khi sửa dữ liệu / MongoDB

- Database name: `velotrax`
- Collection user: `users`
- Index được tạo lúc start cho `users` là `userName` unique và `active` thường.
- User model nằm ở `internal/model/user.go`
- Runtime env đang được code đọc: `GRPC_PORT`, `HTTP_PORT`, `APP_ENV`, `MONGO_URI`, `JWT_SECRET`, `JWT_EXPIRY`, `JWT_REFRESH_EXPIRY`, `ALLOW_ROLE_UPDATE`.

## Khi sửa proto / codegen

- Sửa source proto ở `proto/auth/auth.proto`.
- Sau đó regenerate `internal/gen/auth`.
- Không chỉnh trực tiếp generated code nếu chỉ cần đổi contract.
- Nếu thêm RPC profile hoặc đổi `UserDetail`, cập nhật luôn README và regenerate codegen trước khi test.

## Những file nên tránh đụng nếu không cần

- `internal/gen/auth/*`
- `go.sum` trừ khi dependency thật sự đổi
- File build output, cache, hay artifact sinh ra từ tool

## Kiểm tra sau khi sửa

- `go test ./...`
- Nếu liên quan Docker: `docker compose up --build`
- Nếu đụng config: kiểm tra lại biến môi trường bắt buộc `MONGO_URI` và `JWT_SECRET`
- Nếu có sửa profile flow, thêm kiểm tra nhanh `ALLOW_ROLE_UPDATE` bằng cách set `true` và `false`.
