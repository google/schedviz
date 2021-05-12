# docker build -t schedviz .
# docker run -d -p 127.0.0.1:7402:7402 schedviz

FROM debian

USER root

ARG DEBIAN_FRONTEND=noninteractive
RUN apt-get update && apt-get upgrade -y
RUN apt-get install -y build-essential unzip git
RUN apt-get install -y npm
RUN apt-get install -y libncurses5 libncurses6
RUN apt-get install -y libxml2

RUN npm install -g yarn

RUN git clone https://github.com/google/schedviz.git
RUN cd schedviz && yarn install

RUN cd schedviz && yarn bazel --batch build server:server

RUN mkdir -p /traces
EXPOSE 7402
CMD cd schedviz && yarn bazel run server -- -- -storage_path=/traces
