FROM --platform=${BUILDPLATFORM} golang:1 AS gobuild
ARG TARGETOS
ARG TARGETARCH
WORKDIR /go
COPY backend/ .
RUN GOOS=${TARGETOS} GOARCH=${TARGETARCH} CGO_ENABLED=0 go build -o /go/app

FROM node:24-alpine AS nodebuild
WORKDIR /app
COPY frontend/package*.json .
# ESLint v10 is intentionally used even though some plugins may declare narrower peer ranges.
# Use legacy peer dependency resolution so Docker builds don't fail on ERESOLVE.
RUN npm ci --legacy-peer-deps
COPY frontend/ .
RUN npx vite build

FROM scratch
ARG TARGETOS
ARG TARGETARCH
COPY --from=gobuild /go/app /brave-bpc/brave-bpc.${TARGETARCH}
COPY --from=nodebuild /app/dist /brave-bpc/dist

