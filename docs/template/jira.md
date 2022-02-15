# Create Template for JIRA

Ask to JIRA rest API to get which custom fields are usable.

https://developer.atlassian.com/server/jira/platform/jira-rest-api-examples/#creating-an-issue-examples

### Service Increment

With this query, we see service increment's issuetypeid.

```sh
curl -H "Authorization: Bearer ${TOKEN}" -H "Content-Type: application/json" "https://jira.techno.ingenico.com/rest/api/2/issue/createmeta/LBO/issuetypes" | jq .
```

After that we need to check detail fields.

```sh
curl -H "Authorization: Bearer ${TOKEN}" -H "Content-Type: application/json" "https://jira.techno.ingenico.com/rest/api/2/issue/createmeta/LBO/issuetypes/11707" | jq .
```

Check your issues

```sh
curl -H "Authorization: Bearer ${TOKEN}" -H "Content-Type: application/json" "https://jira.techno.ingenico.com/rest/api/2/search?jql=assignee=eates" | jq .
```

Look an example issue

```sh
curl -H "Authorization: Bearer ${TOKEN}" -H "Content-Type: application/json" "https://jira.techno.ingenico.com/rest/api/2/issue/LBO-72121" | jq .
```

Example: https://jira.techno.ingenico.com/browse/LBO-72121

Click export to XML format button to view datas but with looking api is better.

### Template

Create myitsm ticket

```json
{
  "fields": {
    "project":
    {
      "key": "LBO"
    },
    "reporter": "eates",
    "summary": "Release - Validator to create TRSVALEX event when invalidating a transaction",
    "description": "",
    "issuetype": {
      "name": "Service Increment"
    },
    "priority": {
      "name": "Minor"
    },
    "customfield_10006": {
      "value": "LBO-59087"
    },
    "customfield_11601": {
      "value": "FinOps - DeepCore"
    }
  },
  "update":{
    "issuelinks":[
      {
        "add":{
          "type":{
            "name":"Relates",
          },
          "outwardIssue": {
            "key": "LBO-71558"
          }
        }
      },
      {
        "add":{
          "type":{
            "name":"Relates",
          },
          "outwardIssue": {
            "key": "LBO-71973"
          }
        }
      }
    ]
  }
}
```

Template

```
{
  "fields": {
    "project":
    {
       "key": "LBO"
    },
    "reporter": "{{.reporter}}",
    "summary": "{{.summary}}",
    "description": "{{or .description ""}}",
    "issuetype": {
      "name": "Service Increment"
    },
    "priority": {
      "name": "Minor"
    },
    "customfield_10006": {
      "value": "{{.epic}}"
    },
    "customfield_11601": {
      "value": "FinOps - DeepCore"
    }
  },
  "update":{
    "issuelinks":[
      {{range $i, $value := .issuelinks }}
      {{- if $i}},{{ end }}
      {
        "add":{
          "type":{
            "name":"Relates",
          },
          "outwardIssue": {
            "key": "{{$value}}"
          }
        }
      }
      {{- end}}
    ]
  }
}
```

Values

```yml
summary: Release - Validator to create TRSVALEX event when invalidating a transaction
reporter: eates
epic: LBO-59087
issuelinks:
  - LBO-71558
  - LBO-71973
```

## Test Server

Open test server with `docker run --rm -it --name="whoami" -p 9090:80 traefik/whoami`.

Add an auth entry to show this server.

```json
{"id":"secret","headers":"{\"Authorization\": \"Bearer <token>\"}","URL":"http://localhost:9090","method":"POST"}
```

Add an template and bind it.

```
hello {{.name}}
```

```json
{"id":"sendhi","authentication":"secret","template":"test"}
```

Now send values with curl or in the swagger documentation.

```sh
curl -X 'POST' \
  'http://localhost:3000/api/v1/send?name=sendhi' \
  -H 'accept: application/json' \
  -H 'Authorization: Bearer aaabbbccc'
  -H 'Content-Type: text/plain' \
  -d 'name: test'
```

## Local JIRA for testing

For testing added own jira server. (using 8282 as port number)

```sh
docker run -v jiraVolume:/var/atlassian/application-data/jira --name="jira" -d -p 8282:8080 atlassian/jira-software
```

After that you need to enter a license key to use it.

When installation complete, check jira version and look at the REST-API documentation.

https://docs.atlassian.com/software/jira/docs/api/REST/8.20.1/

In the profile page, add a personal access token.

Use your token with bearer header

```sh
curl -H "Authorization: Bearer MjQ5Nzc3NTg3MjM4OosJndoCMilW9HAnAl4T2CfMEnbG" http://localhost:8282/rest/api/2/issue/SCRM-10
```

Now add auth to chore with giving this header and `POST` method.

https://developer.atlassian.com/cloud/jira/platform/basic-auth-for-rest-apis/