## 1.CreateContract

创建代币合约

- 接口定义

```grpc
rpc CreateContract(req_token_create) returns (resp_token_create) {}
```

- 请求参数（req_token_create）
    序号|字段|类型|描述
    :-:|:--|:--|:--
    1|from|string|代币合约创建者地址
    2|to|string|任意地址
    3|amout|uint64|转账金额，应大于500000
    4|nonce|uint64|随机数
    5|priv|string|私钥
    6|symbol|string|代币名称
    7|total|uint64|代币总数
    8|fee|uint64|手续费，应大于500000
- 响应参数（resp_token_create）
    序号|字段|类型|描述
    :-:|:--|:--|:--
    1|hash|string|交易hash

## 2.MintToken
铸币

- 接口定义
```grpc
rpc MintToken(req_token_create) returns (resp_token_create) {}
```
- 请求参数（req_token_create）
    序号|字段|类型|描述
    :-:|:--|:--|:--
    1|from|string|代币合约创建者地址
    2|to|string|任意地址
    3|amout|uint64|转账金额，应大于500000
    4|nonce|uint64|随机数
    5|priv|string|私钥
    6|symbol|string|代币名称
    7|total|uint64|代币总数
    8|fee|uint64|手续费，应大于500000
- 响应参数（resp_token_create）
    序号|字段|类型|描述
    :-:|:--|:--|:--
    1|hash|string|交易hash

**注意:** 此接口应在CreateContract后调用，并保持参数值一致！

## 3.SendToken
发送代币交易

接口定义：
```grpc
rpc SendToken(req_token_transaction) returns (resp_token_transaction) {}
```
- 请求参数（req_token_transaction）
    序号|字段|类型|描述
    :-:|:--|:--|:--
    1|from|string|代币拥有者的地址
    2|to|string|任意地址
    3|amout|uint64|转账金额，应大于500000
    4|nonce|uint64|随机数
    5|priv|string|私钥
    6|tokenAmount|uint64|代币金额
    7|symbol|string|代币名称
    8|fee|uint64|手续费，应大于500000
- 响应参数（resp_token_transaction）
    序号|字段|类型|描述
    :-:|:--|:--|:--
    1|hash|string|交易hash

## 4.GetBalanceToken
获取代币余额

- 接口定义
```grpc
rpc GetBalanceToken(req_token_balance) returns (resp_token_balance) {}
```
- 请求参数(req_token_balance)
    序号|字段|类型|描述
    :-:|:--|:--|:--
    1|address|string|地址
    2|symbol|string|代币名称
- 响应参数（resp_token_balance）
    序号|字段|类型|描述
    :-:|:--|:--|:--
    1|balnce|uint64|余额

