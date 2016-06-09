# PAKT Protocol

This document defines the PAKT protocol specification.

## Byte Order

All fields are encoded in network order (**big endian**).

## Message Frame

| NAME           | TYPE   | SIZE    |
|:---------------|:-------|:--------|
| Version        | uint8  | 1 byte  |
| Type           | uint8  | 1 byte  |
| Header Length  | uint16 | 2 bytes |
| Payload Length | uint32 | 4 bytes |
| Header Data    | bytes  | -       |
| Payload Data   | bytes  | -       |

### Version Field

The version field is used for future backward compatibility. At the current time, the field is always set to 0, to indicate the initial version.

### Type Field

| VALUE | NAME       | DESCRIPTION                        |
|:------|:-----------|:-----------------------------------|
| 0x0   | Close      | Close the connection               |
| 0x1   | Ping       | Request a pong response.           |
| 0x2   | Pong       | Respond to a ping request.         |
| 0x3   | Call       | Call a remote function             |
| 0x4   | CallReturn | Return from a remote function call |


### Keep-Alive

Each connection peer should request ping messages to check if the connection is still alive.
A ping message should only be requested if the connection is idle. As soon as any valid message is received, the ping and socket timeout timers must be reset.
