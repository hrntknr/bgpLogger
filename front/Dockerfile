FROM node:12

COPY . /app

WORKDIR /app

RUN yarn install && yarn build

CMD [ "yarn", "start" ]
