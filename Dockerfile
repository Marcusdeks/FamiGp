# Etapa 1: compila la interfaz React con Vite.
FROM node:24-alpine AS web
WORKDIR /web
COPY web/package.json web/package-lock.json ./
RUN npm ci
COPY web/ ./
RUN npm run build

# Etapa 2: compila el servidor Go con la interfaz incrustada, como binario
# estático sin dependencias de C.
FROM golang:1.24-alpine AS build
WORKDIR /src
COPY . .
COPY --from=web /web/dist/app ./web/dist/app
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /famigp ./cmd/famigp

# Imagen final mínima: solo el binario, ejecutado con un usuario sin privilegios.
FROM gcr.io/distroless/static-debian12:nonroot
COPY --from=build /famigp /famigp
EXPOSE 8080
ENTRYPOINT ["/famigp", "-addr", ":8080"]
