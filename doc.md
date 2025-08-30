#### GET /orders
Response
```json
{
  "data": [
    {
      "id": "9eae72d9-890f-4a11-87f3-d5632d989493",
      "restaurant_name": "rest_name",
      "table_number": 5,
      "created_at": "2025-09-02T17:13:01.282703Z",
      "updated_at": "2025-09-02T17:13:01.282703Z",
      "accepted": false
    }
  ],
  "page": 1,
  "size": 10,
  "total": 1
}
```
#### GET /orders?page=2&size=1
Response
```json
{
  "data": [
    {
      "id": "f90aa76f-2902-4435-aef6-a3143a376f77",
      "restaurant_name": "rest_name",
      "table_number": 1,
      "created_at": "2025-09-02T17:20:11.841173Z",
      "updated_at": "2025-09-02T17:20:11.841173Z",
      "accepted": false
    }
  ],
  "page": 2,
  "size": 1,
  "total": 4
}
```
#### POST /orders/{orderId}/accept
Response
```json
{
  "orderId": "f90aa76f-2902-4435-aef6-a3143a376f77",
  "status": "ok"
}
```