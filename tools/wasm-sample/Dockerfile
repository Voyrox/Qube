FROM nikolaik/python-nodejs:latest

RUN mkdir -p /usr/src/bot
WORKDIR /usr/src/bot
COPY ["package.json", "npm-shrinkwrap.json*", "./"] /usr/src/bot/

COPY . /usr/src/bot
EXPOSE 3000
USER pn
CMD ["npm i", "node index.js"]