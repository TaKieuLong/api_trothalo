// Package docs Code generated by swaggo/swag. DO NOT EDIT
package docs

import "github.com/swaggo/swag"

const docTemplate = `{
    "schemes": {{ marshal .Schemes }},
    "swagger": "2.0",
    "info": {
        "description": "{{escape .Description}}",
        "title": "{{.Title}}",
        "contact": {},
        "version": "{{.Version}}"
    },
    "host": "{{.Host}}",
    "basePath": "{{.BasePath}}",
    "paths": {
        "/users": {
            "get": {
                "description": "Get a list of all users",
                "produces": [
                    "application/json"
                ],
                "summary": "Get all users",
                "responses": {
                    "200": {
                        "description": "OK",
                        "schema": {
                            "type": "object",
                            "properties": {
                                "code": {
                                    "type": "integer"
                                },
                                "mess": {
                                    "type": "string"
                                },
                                "data": {
                                    "type": "array",
                                    "items": {
                                        "$ref": "#/definitions/models.User"
                                    }
                                }
                            }
                        }
                    }
                }
            },
            "post": {
                "description": "Create a new user",
                "consumes": [
                    "application/json"
                ],
                "produces": [
                    "application/json"
                ],
                "summary": "Create a new user",
                "parameters": [
                    {
                        "description": "User data",
                        "name": "user",
                        "in": "body",
                        "required": true,
                        "schema": {
                            "$ref": "#/definitions/CreateUserRequest"
                        }
                    }
                ],
                "responses": {
                    "201": {
                        "description": "Created",
                        "schema": {
                            "type": "object",
                            "properties": {
                                "code": {
                                    "type": "integer"
                                },
                                "mess": {
                                    "type": "string"
                                },
                                "data": {
                                    "$ref": "#/definitions/models.User"
                                }
                            }
                        }
                    },
                    "400": {
                        "description": "Bad Request",
                        "schema": {
                            "type": "object",
                            "properties": {
                                "code": {
                                    "type": "integer"
                                },
                                "mess": {
                                    "type": "string"
                                }
                            }
                        }
                    },
                    "500": {
                        "description": "Internal Server Error",
                        "schema": {
                            "type": "object",
                            "properties": {
                                "code": {
                                    "type": "integer"
                                },
                                "mess": {
                                    "type": "string"
                                }
                            }
                        }
                    }
                }
            }
        },
        "/users/{id}": {
            "get": {
                "description": "Get a specific user by ID",
                "produces": [
                    "application/json"
                ],
                "summary": "Get a user by ID",
                "parameters": [
                    {
                        "type": "integer",
                        "description": "User ID",
                        "name": "id",
                        "in": "path",
                        "required": true
                    }
                ],
                "responses": {
                    "200": {
                        "description": "OK",
                        "schema": {
                            "type": "object",
                            "properties": {
                                "code": {
                                    "type": "integer"
                                },
                                "mess": {
                                    "type": "string"
                                },
                                "data": {
                                    "$ref": "#/definitions/models.User"
                                }
                            }
                        }
                    },
                    "404": {
                        "description": "User not found",
                        "schema": {
                            "type": "object",
                            "properties": {
                                "code": {
                                    "type": "integer"
                                },
                                "mess": {
                                    "type": "string"
                                }
                            }
                        }
                    }
                }
            },
            "put": {
                "description": "Update a specific user by ID",
                "consumes": [
                    "application/json"
                ],
                "produces": [
                    "application/json"
                ],
                "summary": "Update a user by ID",
                "parameters": [
                    {
                        "type": "integer",
                        "description": "User ID",
                        "name": "id",
                        "in": "path",
                        "required": true
                    },
                    {
                        "description": "User update data",
                        "name": "user",
                        "in": "body",
                        "required": true,
                        "schema": {
                            "$ref": "#/definitions/UpdateUser"
                        }
                    }
                ],
                "responses": {
                    "200": {
                        "description": "OK",
                        "schema": {
                            "type": "object",
                            "properties": {
                                "code": {
                                    "type": "integer"
                                },
                                "mess": {
                                    "type": "string"
                                },
                                "data": {
                                    "$ref": "#/definitions/models.User"
                                }
                            }
                        }
                    },
                    "400": {
                        "description": "Bad Request",
                        "schema": {
                            "type": "object",
                            "properties": {
                                "code": {
                                    "type": "integer"
                                },
                                "mess": {
                                    "type": "string"
                                }
                            }
                        }
                    },
                    "404": {
                        "description": "User not found",
                        "schema": {
                            "type": "object",
                            "properties": {
                                "code": {
                                    "type": "integer"
                                },
                                "mess": {
                                    "type": "string"
                                }
                            }
                        }
                    },
                    "500": {
                        "description": "Internal Server Error",
                        "schema": {
                            "type": "object",
                            "properties": {
                                "code": {
                                    "type": "integer"
                                },
                                "mess": {
                                    "type": "string"
                                }
                            }
                        }
                    }
                }
            },
            "delete": {
                "description": "Delete a specific user by ID",
                "summary": "Delete a user by ID",
                "parameters": [
                    {
                        "type": "integer",
                        "description": "User ID",
                        "name": "id",
                        "in": "path",
                        "required": true
                    }
                ],
                "responses": {
                    "204": {
                        "description": "No Content"
                    },
                    "404": {
                        "description": "User not found",
                        "schema": {
                            "type": "object",
                            "properties": {
                                "code": {
                                    "type": "integer"
                                },
                                "mess": {
                                    "type": "string"
                                }
                            }
                        }
                    }
                }
            }
        },
        "/users/{id}/status": {
            "put": {
                "description": "Change the status of a user by ID",
                "produces": [
                    "application/json"
                ],
                "summary": "Change user status",
                "parameters": [
                    {
                        "type": "integer",
                        "description": "User ID",
                        "name": "id",
                        "in": "path",
                        "required": true
                    },
                    {
                        "type": "integer",
                        "description": "New status",
                        "name": "status",
                        "in": "query",
                        "required": true
                    }
                ],
                "responses": {
                    "200": {
                        "description": "OK",
                        "schema": {
                            "type": "object",
                            "properties": {
                                "code": {
                                    "type": "integer"
                                },
                                "mess": {
                                    "type": "string"
                                },
                                "data": {
                                    "$ref": "#/definitions/models.User"
                                }
                            }
                        }
                    },
                    "400": {
                        "description": "Bad Request",
                        "schema": {
                            "type": "object",
                            "properties": {
                                "code": {
                                    "type": "integer"
                                },
                                "mess": {
                                    "type": "string"
                                }
                            }
                        }
                    },
                    "404": {
                        "description": "User not found",
                        "schema": {
                            "type": "object",
                            "properties": {
                                "code": {
                                    "type": "integer"
                                },
                                "mess": {
                                    "type": "string"
                                }
                            }
                        }
                    },
                    "500": {
                        "description": "Internal Server Error",
                        "schema": {
                            "type": "object",
                            "properties": {
                                "code": {
                                    "type": "integer"
                                },
                                "mess": {
                                    "type": "string"
                                }
                            }
                        }
                    }
                }
            }
        }
    },
    "definitions": {
        "models.User": {
            "type": "object",
            "properties": {
                "id": {
                    "type": "integer"
                },
                "email": {
                    "type": "string"
                },
                "name": {
                    "type": "string"
                },
                "avatar": {
                    "type": "string"
                },
                "role": {
                    "type": "integer"
                },
                "status": {
                    "type": "integer"
                },
                "created_at": {
                    "type": "string",
                    "format": "date-time"
                },
                "updated_at": {
                    "type": "string",
                    "format": "date-time"
                }
            }
        },
        "CreateUserRequest": {
            "type": "object",
            "properties": {
                "email": {
                    "type": "string"
                },
                "name": {
                    "type": "string"
                },
                "avatar": {
                    "type": "string"
                },
                "role": {
                    "type": "integer"
                }
            }
        },
        "UpdateUser": {
            "type": "object",
            "properties": {
                "name": {
                    "type": "string"
                },
                "phoneNumber": {
                    "type": "string"
                },
                "avatar": {
                    "type": "string"
                },
                "role": {
                    "type": "integer"
                },
                "status": {
                    "type": "integer"
                }
            }
        }
    }
}`

// SwaggerInfo holds exported Swagger Info so clients can modify it
var SwaggerInfo = &swag.Spec{
    Version:          "1.0",
    Host:             "localhost:8083",
    BasePath:         "/api/v1",
    Schemes:          []string{"http"},
    Title:            "User Management API",
    Description:      "API for managing users in the application.",
	InfoInstanceName: "swagger",
	SwaggerTemplate:  docTemplate,
	LeftDelim:        "{{",
	RightDelim:       "}}",
}

func init() {
	swag.Register(SwaggerInfo.InstanceName(), SwaggerInfo)
}
