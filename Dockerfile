FROM public.ecr.aws/docker/library/golang:1.20-alpine AS builder

COPY . .
RUN CGO_ENABLED=0 go build -o /kep3633alt main.go \
    && chmod +x /kep3633alt \
    ;

FROM public.ecr.aws/docker/library/alpine:latest AS runner

COPY --from=builder --chmod /kep3633alt /kep3633alt

CMD["/kep3633alt"]
