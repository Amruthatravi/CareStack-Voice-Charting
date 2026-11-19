FROM node:20-alpine AS build
WORKDIR /app
RUN apk add --no-cache curl unzip
COPY package.json package-lock.json ./
RUN npm ci --no-audit --no-fund
COPY . .
# Bake the on-device Vosk speech model into the image (served statically, runs in the browser).
RUN sh scripts/get-model.sh && npm run build

FROM nginx:1.27-alpine
COPY nginx.conf /etc/nginx/conf.d/default.conf
COPY --from=build /app/dist /usr/share/nginx/html
EXPOSE 80
