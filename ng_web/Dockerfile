FROM node:22-alpine

WORKDIR /app

# On installe Angular CLI globalement
RUN npm install -g @angular/cli@21

COPY package*.json ./
RUN npm install

COPY . .

# On expose le port 4200 (Angular)
EXPOSE 4200

# Démarrer le serveur dev avec écoute sur toutes les interfaces
CMD ["npm", "start", "--", "--host", "0.0.0.0", "--poll", "2000"]
