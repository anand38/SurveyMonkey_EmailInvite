package sendEmailInvite

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"net/http"
	"strings"
	"time"

	"github.com/TIBCOSoftware/flogo-lib/core/activity"
	"github.com/tidwall/gjson"
	"github.com/tidwall/sjson"
)

// MyActivity is a stub for your Activity implementation
type MyActivity struct {
	metadata *activity.Metadata
}

// NewActivity creates a new activity
func NewActivity(metadata *activity.Metadata) activity.Activity {
	return &MyActivity{metadata: metadata}
}

// Metadata implements activity.Activity.Metadata
func (a *MyActivity) Metadata() *activity.Metadata {
	return a.metadata
}
func callUrl(method string, url string, bodyContent *bytes.Buffer, accessToken string) string {

	request, _ := http.NewRequest(method, url, bodyContent)
	request.Header.Set("Authorization", "bearer "+accessToken)
	request.Header.Set("Content-Type", "application/json")
	fmt.Println(bodyContent)
	client := &http.Client{}
	succ_resp, error_resp := client.Do(request)
	if error_resp != nil {
		outResult := `{ "Error" : { "message" : "The HTTP request for getting SurveyID failed with error ` + error_resp.Error() + `" } }`
		//context.SetOutput("status", outResult)
		return outResult
	} else {
		survey_response, _ := ioutil.ReadAll(succ_resp.Body)
		hasError := gjson.Get(string(survey_response), "error.http_status_code").Int()
		if hasError == 400 || hasError == 401 || hasError == 404 {
			outResult := `{ "Error" : { "message" : ` + gjson.Get(string(survey_response), "error.message").String() + ` } }`
			fmt.Println(outResult)
			//context.SetOutput("status", outResult)
			return outResult
		} else {
			return string(survey_response)
		}
	}
}

// Eval implements activity.Activity.Eval
func (a *MyActivity) Eval(context activity.Context) (done bool, err error) {
	// do eval
	accessToken := context.GetInput("AuthToken").(string)
	surveyName := context.GetInput("Survey Name").(string)
	senderEmail := context.GetInput("Sender's Email").(string)
	recipientList := context.GetInput("recipientList").(string)
	typeofEmail := context.GetInput("type").(string)
	// recipientStatus := context.GetInput("recipient_status").(string)
	subject := context.GetInput("Subject").(string)
	body := context.GetInput("Body").(string)
	surveyID := ""
	method := ""
	containError := ""

	//get surveyId : API Call #1
	method = "GET"
	surveyIdurl := "https://api.surveymonkey.com/v3/surveys?title=" + surveyName
	var jsonBody = []byte("")
	reqSurveyID := callUrl(method, surveyIdurl, bytes.NewBuffer(jsonBody), accessToken)
	//fmt.Println(reqSurveyID)
	containError = gjson.Get(string(reqSurveyID), "Error").String()
	if containError != "" {
		context.SetOutput("status", containError)
		return true, nil
	} else {
		surveyID = gjson.Get(reqSurveyID, "data.0.id").String()
		fmt.Println("surveyId: " + surveyID)
	}

	//set email invite and get Collector id , API Call #2
	method = "POST"
	collectorID := ""
	collector_url := "https://api.surveymonkey.com/v3/surveys/" + surveyID + "/collectors"
	jsonBody = []byte(`{"type":"email","sender_email":"` + senderEmail + `"}`)
	reqCollectorID := callUrl(method, collector_url, bytes.NewBuffer(jsonBody), accessToken)
	//	fmt.Println(reqCollectorID)
	containError = gjson.Get(string(reqSurveyID), "Error").String()
	if containError != "" {
		context.SetOutput("status", containError)
		return true, nil
	} else {
		collectorID = gjson.Get(reqCollectorID, "id").String()
		fmt.Println("Collector ID: " + collectorID)
	}

	//get message ID , APICall #3
	method = "POST"
	messageID := ""
	message_url := "https://api.surveymonkey.com/v3/collectors/" + collectorID + "/messages"
	if typeofEmail == "invite" {
		if body != "" {
			surveyLink := "[SurveyLink]"
			optLink := "[OptOutLink]"
			footerLink := "[FooterLink]"
			jsonBody = []byte(`{"type":"invite","subject":"` + subject + `","body_text":"` + body + "<a href=" + "\\" + "\"" + surveyLink + "\\" + "\"" + " >Take the survey!</a> <a href=" + "\\" + "\"" + optLink + "\\" + "\"" + ">Please remove me from your mailing list.</a> <a href=" + "\\" + "\"" + footerLink + "\\" + "\"" + ">Footer!</a>" + `"}`)
		} else {
			jsonBody = []byte(`{"type":"invite","subject":"` + subject + `"}`)
		}
		reqMessageID := callUrl(method, message_url, bytes.NewBuffer(jsonBody), accessToken)
		containError = gjson.Get(string(reqMessageID), "Error").String()
		if containError != "" {
			context.SetOutput("status", containError)
			return true, nil
		} else {
			messageID = gjson.Get(reqMessageID, "id").String()
			fmt.Println("Message ID: " + messageID)
		}

		//add multiple email ids , API Call #4
		method = "POST"
		recipient_url := "https://api.surveymonkey.com/v3/collectors/" + collectorID + "/messages/" + messageID + "/recipients/bulk"
		emailParent_Json := `{ "contacts": [`
		emails := strings.Split(recipientList, ",")
		count := 0
		for i := 0; i < len(emails); i++ {
			innerJsonContent, _ := sjson.Set("", "email", emails[i])
			emailParent_Json = emailParent_Json + innerJsonContent
			if count < len(emails)-1 {
				count = count + 1
				emailParent_Json = emailParent_Json + ","
			}
		}
		emailParent_Json = emailParent_Json + "]}"
		jsonBody = []byte(emailParent_Json)
		reqRecipientBulk := callUrl(method, recipient_url, bytes.NewBuffer(jsonBody), accessToken)
		containError = gjson.Get(string(reqRecipientBulk), "Error").String()
		if containError != "" {
			context.SetOutput("status", containError)
			return true, nil
		} else {
			succStatus_1 := gjson.Get(reqRecipientBulk, "succeeded.#").String()
			succStatus_2 := gjson.Get(reqRecipientBulk, "existing.#").String()
			if succStatus_1 != "" || succStatus_2 != "" {
				fmt.Println("emails added successfully...")
			} else {
				context.SetOutput("status", `{ "Error" : { "message" : "Error while adding recipients emails." }}`)
				return true, nil
			}
		}
	}

	//add schedule email date , API Call #5
	method = "POST"
	send_url := "https://api.surveymonkey.com/v3/collectors/" + collectorID + "/messages/" + messageID + "/send"
	curr_date := time.Now().Add(-328 * time.Minute).Format("2006-01-02T15:04:05+00:00") //SUTRACTING 5:28 hrs FROM CURRENT TIMESTAMP SO THAT EMAIL CAN BE SENT AFTRE 2 MIN AFTER THE REQUEST HITS
	jsonBody = []byte(`{ "scheduled_date": "` + curr_date + `"}`)
	reqSendMail := callUrl(method, send_url, bytes.NewBuffer(jsonBody), accessToken)
	containError = gjson.Get(string(reqSendMail), "Error").String()
	if containError != "" {
		context.SetOutput("status", containError)
		return true, nil
	} else {
		fmt.Println("Email sent succesfully")
	}
	return true, nil
}
