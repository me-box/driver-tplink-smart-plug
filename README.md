# driver-tplink-smart-plug

A driver to collect data from TP-Link smart plugs

# Usage

Install the driver on data box then configure your local subnet.

By default, it will scan 192.168.1.[1-254] for plugs but this is configurable.

# Data

For each plug there are two data sources

1) TP-Power-Usage, this holds a time series of:
```
    data: {
            "current":0.01587,
            "voltage":300.422028,
            "power":0,
            "total":0.017,
            "err_code":0
        }
```
2) TP-PowerState, this holds a time series of
```
    data: {"state":1}

    1 indicates on 0 indicates off
```

and one actuator

1) TP-SetPowerState, write on or off to turn the plug on or off!

## Databox is funded by the following grants:

```
EP/N028260/1, Databox: Privacy-Aware Infrastructure for Managing Personal Data

EP/N028260/2, Databox: Privacy-Aware Infrastructure for Managing Personal Data

EP/N014243/1, Future Everyday Interaction with the Autonomous Internet of Things

EP/M001636/1, Privacy-by-Design: Building Accountability into the Internet of Things (IoTDatabox)

EP/M02315X/1, From Human Data to Personal Experience

```