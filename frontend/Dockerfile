#Build stage
FROM --platform=$TARGETPLATFORM node:23-alpine AS base
WORKDIR /app
COPY package*.json ./
RUN npm ci
COPY . .

#Dev stage for running VITE in test mode
FROM base AS dev
EXPOSE 3000
CMD ["npm", "run", "dev"]

# Build the static SPA for production
FROM base AS build
RUN npm run build

# Production stage: nginx serves the SPA and proxies backend routes
FROM nginx:1.27-alpine AS prod
COPY nginx.conf /etc/nginx/conf.d/default.conf
COPY --from=build /app/dist /usr/share/nginx/html
EXPOSE 80
CMD ["nginx", "-g", "daemon off;"]