# W3bstream

## Overview

W3bStream is a general framework for connecting data generated by devices and machines in the physical world to the blockchain world. In a nutshell, W3bStream uses the IoTeX blockchain to orchestrate a decentralized network of gateways (i.e., W3bStream nodes) that streams [encrypted] data from IoT devices and machines and generates proofs of real-world facts to different blockchains. 

![image](https://user-images.githubusercontent.com/448293/196618039-365ab2b7-f50a-49c8-a02d-c28e48acafcb.png)


## Arch

![w3bstream](__doc__/modules_and_dataflow.png)


## Run with docker

### init frontend

```bash
make init_frontend
```

### Update frontend to latest if needed

```bash
make update_frontend
```

### Build docker image

```bash
make build_image
```

### Run docker container

```bash
 make run_image
 ```

 ### drop docker image
 ```bash
 make drop_image
 ```

### Access W3bstream Studio

Visit http://localhost:3000 to get started.

The default admin password is `iotex.W3B.admin`

## Run with binary

### Dependencies:

- OS : macOS(11.0+) / Linux (tested on Ubuntu 16+)
- Docker: to start a postgres
- Httpie: a simple curl command (used to interact with W3bstream node via cli)
- Tinygo: to build wasm code

### Init protocols and database

```sh
make run_depends # start postgres and mqtt
make migrate     # create or update schema
```

### Start a server

```sh
make run_server
```

keep the terminal alive, and open a new terminal for the other commands.

### Login (fetch auth token)

command

```sh
echo '{"username":"admin","password":"${password}"}' | http put :8888/srv-applet-mgr/v0/login
```

output like

```json
{
  "accountID": "${account_id}",
  "expireAt": "2022-09-23T07:20:08.099601+08:00",
  "issuer": "srv-applet-mgr",
  "token": "${token}"
}
```

```sh
export TOK=${token}
```

### Create your project

command

```sh
echo '{"name":"${project_name}"}' | http :8888/srv-applet-mgr/v0/project -A bearer -a $TOK
```

output like

```json
{
  "accountID": "${account_id}",
  "createdAt": "2022-10-14T12:50:26.890393+08:00",
  "name": "${project_name}",
  "projectID": "${project_id}",
  "updatedAt": "2022-10-14T12:50:26.890407+08:00"
}
```

### Build demo wasm scripts

```sh
make wasm_demo ## build to `examples` use to deploy wasm applet
```

### Create and deploy applet

upload wasm script

> use `examples/word_count/word_count.wasm` or `examples/log/log.wasm`

```sh
## set env vars
export PROJECTID=${project_id}
export PROJECTNAME=${project_name}
export WASMFILE=examples/log/log.wasm
http --form post :8888/srv-applet-mgr/v0/applet/$PROJECTID file@$WASMFILE info='{"appletName":"log","strategies":[{"eventType":"ANY","handler":"start"}]}' -A bearer -a $TOK
```

output like

```json
{
  "appletID": "${apple_id}",
  "createdAt": "2022-10-14T12:53:10.590926+08:00",
  "name": "${applet_name}",
  "projectID": "${project_id}",
  "updatedAt": "2022-10-14T12:53:10.590926+08:00"
}
```

deploy applet

```sh
export APPLETID=${applet_id}
http post :8888/srv-applet-mgr/v0/deploy/applet/$APPLETID -A bearer -a $TOK
```

output like

```json
{
  "instanceID": "${instance_id}",
  "instanceState": "CREATED"
}
```

start applet

```sh
export INSTANCEID=${instance_id}
http put :8888/srv-applet-mgr/v0/deploy/$INSTANCEID/START -A bearer -a $TOK
```

### Register publisher

```sh
export PUBNAME=${publisher_name}
export PUBKEY=${publisher_unique_key} # global unique
echo '{"name":"'$PUBNAME'", "key":"'$PUBKEY'"}' | http post :8888/srv-applet-mgr/v0/publisher/$PROJECTID -A bearer -a $TOK
```

output like

```sh
{
    "createdAt": "2022-10-16T12:28:49.628716+08:00",
    "key": "${publisher_unique_key}",
    "name": "${publisher_name}",
    "projectID": "935772081365103",
    "publisherID": "${pub_id}",
    "token": "${pub_token}",
    "updatedAt": "2022-10-16T12:28:49.628716+08:00"
}
```

### Publish event to server by http

```sh
export PUBTOKEN=${pub_token}
export EVENTTYPE=2147483647 # 0x7FFFFFFF means any type
echo '{"header":{"event_type":'$EVENTTYPE',"pub_id":"'$PUBKEY'","pub_time":'`date +%s`',"token":"'$PUBTOKEN'"},"payload":"xxx yyy zzz"}' | http post :8888/srv-applet-mgr/v0/event/$PROJECTNAME
```
echo '{"header":{"event_type":'$EVENTTYPE',"pub_id":"'$PUBKEY'","pub_time":'$PUBTIME',"token":"'$PUBTOKEN'"},"payload":"xxx yyy zzz"}' | http post :8888/srv-applet-mgr/v0/event/$PROJECTNAME

output like

```json
[
  {
    "instanceID": "${instance_id}",
    "resultCode": 0
  }
]
```

that means some instance handled this event successfully

### Publish event to server through MQTT

- make publishing client

```sh
make build_pub_client
```

- try to publish a message

* event json message

```json
{
  "header": {
    "event_type": '$EVENTTYPE',
    "pub_id": "'$PUBKEY'",
    "pub_time": '`date +%s`',
    "token": "'$PUBTOKEN'"
  },
  "payload": "xxx yyy zzz"
}
```

* event_type: 0x7FFFFFFF any type
* pub_id: the unique publisher id assiged when publisher registering
* token: empty if dont have
* pub_time: timestamp when message published

```sh
# -c means published content
# -t means mqtt topic, the target project name created before
cd build && ./pub_client -c '{"header":{"event_type":'$EVENTTYPE',"pub_id":"'$PUBKEY'","pub_time":'`date +%s`',"token":"'$PUBTOKEN'"},"payload":"xxx yyy zzz"}' -t $PROJECTNAME
```

server log like

```json
{
  "@lv": "info",
  "@prj": "srv-applet-mgr",
  "@ts": "20221017-092252.877+08:00",
  "msg": "sub handled",
  "payload": {
    "payload": "xxx yyy zzz"
  }
}
```


### Post blockchain contract event log monitor 

```sh
echo '{"contractlog":{"chainID": 4690, "contractAddress": "${contractAddress}","blockStart": ${blockStart},"blockEnd": ${blockEnd},"topic0":"${topic0}"}}' | http :8888/srv-applet-mgr/v0/project/monitor/$PROJECTID -A bearer -a $TOK
```

output like

```json
{
    "blockCurrent": ${blockCurrent},
    "blockEnd": ${blockEnd},
    "blockStart": ${blockStart},
    "chainID": 4690,
    "contractAddress": "${contractAddress}",
    "contractlogID": "2162022028435556",
    "createdAt": "2022-10-19T21:21:30.220198+08:00",
    "eventType": "ANY",
    "projectName": "${projectName}",
    "topic0": "${topic0}",
    "updatedAt": "2022-10-19T21:21:30.220198+08:00"
}
```
