resource "langsmith_tool" "lookup_customer" {
  handle      = "lookup-customer"
  name        = "Lookup customer by email"
  description = "Returns customer record fields for a given email address."

  parameters = jsonencode({
    type = "object"
    properties = {
      email = {
        type        = "string"
        description = "Customer email address."
      }
    }
    required = ["email"]
  })

  returns = jsonencode({
    type = "object"
    properties = {
      id         = { type = "string" }
      created_at = { type = "string", format = "date-time" }
    }
  })

  enabled = true
}
