FROM golang:1.20.4-alpine3.17
# working dir default is set to /src , if folder is not there , it will create one
WORKDIR /src

# copg mod and sum file in the default dir 
COPY go.mod go.sum ./
RUN go mod download
# copy all content from src(where the docker build is running, see the makefile to see the host dir of docker build command)
# to the destination dir thaat is the /src in the docker image 
COPY . .

# CGO_ENABLED is 0 to not include c compiler during go buid and rum main.go file from /cmd and store the exe file with name app in /bin
RUN CGO_ENABLED=0 go build -o /bin/app ./cmd

# now this is secon stage where we will get the result of above image and will use alpine linux image to take over and run app.exe after copying neccassary files
FROM alpine
WORKDIR /src

# copy from above container from golang-alpine to new container from alpine image
COPY --from=0 /bin/app /bin/app
COPY --from=0 /src/views /src/views
COPY --from=0 /src/assets /src/assets

ENTRYPOINT [/bin/app]
