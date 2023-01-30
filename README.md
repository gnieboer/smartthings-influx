# smartthings-influx

A simple program to bring to Influx your SmartThings data through the SmartThings API. No SmartApp installation needed.

This is a fork from eargollo/smartthings-influx.  This version assumes you have an existing InfluxDB and Grafana installation that are
up and running and do not need to be replaced.  This is useful if migrating from the smartthings influx app.

## Getting started

If you have Docker, you can get started in just 6 steps.

- Step 1: Create an SmartThings API token

Go to [SmartThings API Token](https://account.smartthings.com/tokens) page and create a token. Make sure you give full access to devices.

- Step 2: Copy the .smarthings-influx.sample.yaml to .smartthings-influx.yaml and save in a location accessible to your Docker runtime

- Step 3: Modify the yaml file to include your API token, and the location / credentials for your existing InfluxDB instance
          (Alternatively, you can use environment variables during container creation, but you still need to map to this file)

- Step 4: Pull the docker container gcndevelopment/smartthingsinflux (note the lack of hyphen)

- Step 5: Create a container and map a folder from where you saved the config file to /config in the container

- Step 6: Run and watch the data come in.  If you run both groovy and this simultaneously, you can filter on entries from here by filtering
          on tag "source" = "docker"

Run Docker Compose:
```
$ UID=$(id -u) GID=$(id -g) docker-compose up
```

Have fun!

## Running locally (requires Golang)

Build the executable
```
$ make build
```

Create the `.smartthings-influx.yaml` file either at your home folder, /config, or at the folder where you run the program:

```yaml
apitoken: <put your SmartThings API token here or export APITOKEN env var>
monitor:
  - light
  - temperatureMeasurement
  - illuminanceMeasurement
  - relativeHumidityMeasurement
period: 120
influxurl: http://localhost:8086
influxuser: user
influxpassword: password
influxdatabase: database
valuemap:
  switch: 
    off: 0
    on: 1
```

You can still monitor non numerical values by adding a value conversion map. On the file above, light has a switch metric whose values are either 'on' or 'off'. The `valuemap` configuration converts on to 1 and off to 0.

If you don't know the capability and their metrics, you can run the `status` option to list the capability and check the monitor error message. It will shwo the missing metrics and their values.

Run the monitor option:
```
$ smartthings-influx monitor
```

You can always set the APITOKEN environment variable in case you don't want your token at the configuration file.
