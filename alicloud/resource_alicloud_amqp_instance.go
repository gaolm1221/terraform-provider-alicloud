package alicloud

import (
	"fmt"
	"log"
	"time"

	util "github.com/alibabacloud-go/tea-utils/service"
	"github.com/aliyun/terraform-provider-alicloud/alicloud/connectivity"
	"github.com/hashicorp/terraform-plugin-sdk/helper/resource"
	"github.com/hashicorp/terraform-plugin-sdk/helper/schema"
	"github.com/hashicorp/terraform-plugin-sdk/helper/validation"
)

func resourceAlicloudAmqpInstance() *schema.Resource {
	return &schema.Resource{
		Create: resourceAlicloudAmqpInstanceCreate,
		Read:   resourceAlicloudAmqpInstanceRead,
		Update: resourceAlicloudAmqpInstanceUpdate,
		Delete: resourceAlicloudAmqpInstanceDelete,
		Importer: &schema.ResourceImporter{
			State: schema.ImportStatePassthrough,
		},
		Timeouts: &schema.ResourceTimeout{
			Create: schema.DefaultTimeout(3 * time.Hour),
		},
		Schema: map[string]*schema.Schema{
			"instance_type": {
				Type:         schema.TypeString,
				Required:     true,
				ForceNew:     true,
				ValidateFunc: validation.StringInSlice([]string{"professional", "vip"}, false),
			},
			"logistics": {
				Type:     schema.TypeString,
				Optional: true,
			},
			"max_eip_tps": {
				Type:     schema.TypeString,
				Optional: true,
				Computed: true,
				DiffSuppressFunc: func(k, old, new string, d *schema.ResourceData) bool {
					v, ok := d.GetOkExists("support_eip")
					return !(ok && v.(bool))
				},
			},
			"max_tps": {
				Type:     schema.TypeString,
				Required: true,
			},
			"modify_type": {
				Type:         schema.TypeString,
				Optional:     true,
				ValidateFunc: validation.StringInSlice([]string{"Downgrade", "Upgrade"}, false),
			},
			"payment_type": {
				Type:         schema.TypeString,
				Required:     true,
				ValidateFunc: validation.StringInSlice([]string{"Subscription"}, false),
			},
			"period": {
				Type:             schema.TypeInt,
				Optional:         true,
				ValidateFunc:     validation.IntInSlice([]int{1, 12, 2, 24, 3, 6}),
				DiffSuppressFunc: PostPaidDiffSuppressFunc,
			},
			"queue_capacity": {
				Type:     schema.TypeString,
				Required: true,
			},
			"renewal_duration": {
				Type:             schema.TypeInt,
				Optional:         true,
				ValidateFunc:     validation.IntInSlice([]int{1, 12, 2, 3, 6}),
				DiffSuppressFunc: PostPaidAndRenewalDiffSuppressFunc,
			},
			"renewal_duration_unit": {
				Type:             schema.TypeString,
				Optional:         true,
				ValidateFunc:     validation.StringInSlice([]string{"Month", "Year"}, false),
				DiffSuppressFunc: PostPaidAndRenewalDiffSuppressFunc,
			},
			"renewal_status": {
				Type:             schema.TypeString,
				Optional:         true,
				Computed:         true,
				ValidateFunc:     validation.StringInSlice([]string{"AutoRenewal", "ManualRenewal", "NotRenewal"}, false),
				DiffSuppressFunc: PostPaidDiffSuppressFunc,
			},
			"status": {
				Type:     schema.TypeString,
				Computed: true,
			},
			"storage_size": {
				Type:     schema.TypeString,
				Optional: true,
				DiffSuppressFunc: func(k, old, new string, d *schema.ResourceData) bool {
					return d.Get("instance_type").(string) == "professional"
				},
			},
			"support_eip": {
				Type:     schema.TypeBool,
				Required: true,
			},
		},
	}
}

func resourceAlicloudAmqpInstanceCreate(d *schema.ResourceData, meta interface{}) error {
	client := meta.(*connectivity.AliyunClient)
	var response map[string]interface{}
	action := "CreateInstance"
	request := make(map[string]interface{})
	conn, err := client.NewBssopenapiClient()
	if err != nil {
		return WrapError(err)
	}
	parameterMapList := make([]map[string]interface{}, 0)
	parameterMapList = append(parameterMapList, map[string]interface{}{
		"Code":  "Region",
		"Value": client.RegionId,
	})
	if v, ok := d.GetOk("instance_type"); ok {
		parameterMapList = append(parameterMapList, map[string]interface{}{
			"Code":  "InstanceType",
			"Value": v,
		})
	}
	if v, ok := d.GetOk("max_eip_tps"); ok {
		parameterMapList = append(parameterMapList, map[string]interface{}{
			"Code":  "MaxEipTps",
			"Value": v,
		})
	}
	if v, ok := d.GetOk("max_tps"); ok {
		parameterMapList = append(parameterMapList, map[string]interface{}{
			"Code":  "MaxTps",
			"Value": v,
		})
	}
	if v, ok := d.GetOk("queue_capacity"); ok {
		parameterMapList = append(parameterMapList, map[string]interface{}{
			"Code":  "QueueCapacity",
			"Value": v,
		})
	}
	if v, ok := d.GetOkExists("support_eip"); ok {
		parameterMapList = append(parameterMapList, map[string]interface{}{
			"Code":  "SupportEip",
			"Value": convertAmqpInstanceSupportEipRequest(v),
		})
	}
	if v, ok := d.GetOkExists("storage_size"); ok {
		parameterMapList = append(parameterMapList, map[string]interface{}{
			"Code":  "StorageSize",
			"Value": v,
		})
	}
	request["Parameter"] = parameterMapList
	request["SubscriptionType"] = d.Get("payment_type")
	request["ProductCode"] = "ons"
	request["ProductType"] = "ons_onsproxy_pre"

	if v, ok := d.GetOk("logistics"); ok {
		request["Logistics"] = v
	}
	if v, ok := d.GetOk("period"); ok {
		request["Period"] = v
	}

	if v, ok := d.GetOk("renewal_duration"); ok {
		request["RenewPeriod"] = v
	}
	if v, ok := d.GetOk("renewal_status"); ok {
		request["RenewalStatus"] = v
	}
	request["ClientToken"] = buildClientToken("CreateInstance")
	runtime := util.RuntimeOptions{}
	runtime.SetAutoretry(true)
	wait := incrementalWait(3*time.Second, 3*time.Second)
	err = resource.Retry(d.Timeout(schema.TimeoutCreate), func() *resource.RetryError {
		response, err = conn.DoRequest(StringPointer(action), nil, StringPointer("POST"), StringPointer("2017-12-14"), StringPointer("AK"), nil, request, &runtime)
		if err != nil {
			if NeedRetry(err) {
				wait()
				return resource.RetryableError(err)
			}
			if IsExpectedErrors(err, []string{"NotApplicable"}) {
				conn.Endpoint = String(connectivity.BssOpenAPIEndpointInternational)
				return resource.RetryableError(err)
			}
			return resource.NonRetryableError(err)
		}
		return nil
	})
	addDebug(action, response, request)
	if err != nil {
		return WrapErrorf(err, DefaultErrorMsg, "alicloud_amqp_instance", action, AlibabaCloudSdkGoERROR)
	}
	if fmt.Sprint(response["Code"]) != "Success" {
		return WrapError(fmt.Errorf("%s failed, response: %v", action, response))
	}
	responseData := response["Data"].(map[string]interface{})
	d.SetId(fmt.Sprint(responseData["InstanceId"]))
	amqpOpenService := AmqpOpenService{client}
	stateConf := BuildStateConf([]string{}, []string{"SERVING"}, d.Timeout(schema.TimeoutCreate), 5*time.Second, amqpOpenService.AmqpInstanceStateRefreshFunc(d.Id(), []string{"Failed"}))
	if _, err := stateConf.WaitForState(); err != nil {
		return WrapErrorf(err, IdMsg, d.Id())
	}

	return resourceAlicloudAmqpInstanceRead(d, meta)
}
func resourceAlicloudAmqpInstanceRead(d *schema.ResourceData, meta interface{}) error {
	client := meta.(*connectivity.AliyunClient)
	amqpOpenService := AmqpOpenService{client}
	object, err := amqpOpenService.DescribeAmqpInstance(d.Id())
	if err != nil {
		if NotFoundError(err) {
			log.Printf("[DEBUG] Resource alicloud_amqp_instance amqpOpenService.DescribeAmqpInstance Failed!!! %s", err)
			d.SetId("")
			return nil
		}
		return WrapError(err)
	}
	d.Set("instance_type", convertAmqpInstanceTypeResponse(object["InstanceType"]))
	d.Set("status", object["Status"])
	d.Set("support_eip", object["SupportEIP"])
	bssOpenApiService := BssOpenApiService{client}
	queryAvailableInstancesObject, err := bssOpenApiService.QueryAvailableInstances(d.Id(), "ons", "ons_onsproxy_pre")
	if err != nil {
		return WrapError(err)
	}
	d.Set("payment_type", queryAvailableInstancesObject["SubscriptionType"])
	if v, ok := queryAvailableInstancesObject["RenewalDuration"]; ok {
		d.Set("renewal_duration", formatInt(v))
	}
	d.Set("renewal_duration_unit", convertAmqpInstanceRenewalDurationUnitResponse(object["RenewalDurationUnit"]))
	d.Set("renewal_status", queryAvailableInstancesObject["RenewStatus"])
	return nil
}
func resourceAlicloudAmqpInstanceUpdate(d *schema.ResourceData, meta interface{}) error {
	client := meta.(*connectivity.AliyunClient)
	var response map[string]interface{}
	d.Partial(true)

	update := false
	request := map[string]interface{}{
		"InstanceIDs": d.Id(),
	}
	if d.HasChange("renewal_status") {
		update = true
	}
	request["RenewalStatus"] = d.Get("renewal_status")
	if d.HasChange("payment_type") {
		update = true
		request["SubscriptionType"] = d.Get("payment_type")
	}
	request["ProductCode"] = "ons"
	request["ProductType"] = "ons_onsproxy_pre"
	if d.HasChange("renewal_duration") {
		update = true
		request["RenewalPeriod"] = d.Get("renewal_duration")
	}
	if d.HasChange("renewal_duration_unit") {
		update = true
		request["RenewalPeriodUnit"] = convertAmqpInstanceRenewalDurationUnitRequest(d.Get("renewal_duration_unit").(string))
	}
	if update {
		action := "SetRenewal"
		conn, err := client.NewBssopenapiClient()
		if err != nil {
			return WrapError(err)
		}
		wait := incrementalWait(3*time.Second, 3*time.Second)
		err = resource.Retry(d.Timeout(schema.TimeoutUpdate), func() *resource.RetryError {
			response, err = conn.DoRequest(StringPointer(action), nil, StringPointer("POST"), StringPointer("2017-12-14"), StringPointer("AK"), nil, request, &util.RuntimeOptions{})
			if err != nil {
				if NeedRetry(err) {
					wait()
					return resource.RetryableError(err)
				}
				if IsExpectedErrors(err, []string{"NotApplicable"}) {
					conn.Endpoint = String(connectivity.BssOpenAPIEndpointInternational)
					return resource.RetryableError(err)
				}
				return resource.NonRetryableError(err)
			}
			return nil
		})
		addDebug(action, response, request)
		if err != nil {
			return WrapErrorf(err, DefaultErrorMsg, d.Id(), action, AlibabaCloudSdkGoERROR)
		}
		if fmt.Sprint(response["Code"]) != "Success" {
			return WrapError(fmt.Errorf("%s failed, response: %v", action, response))
		}
		d.SetPartial("renewal_status")
		d.SetPartial("payment_type")
		d.SetPartial("renewal_duration")
		d.SetPartial("renewal_duration_unit")
	}
	update = false
	modifyInstanceReq := map[string]interface{}{
		"InstanceId": d.Id(),
	}
	if d.HasChange("max_eip_tps") {
		update = true
	}
	parameterMapList := make([]map[string]interface{}, 0)
	if v, ok := d.GetOk("max_eip_tps"); ok {
		parameterMapList = append(parameterMapList, map[string]interface{}{
			"Code":  "MaxEipTps",
			"Value": v,
		})
	} else if v, ok := d.GetOkExists("support_eip"); !ok || v.(bool) {
		return WrapError(fmt.Errorf(RequiredWhenMsg, "max_eip_tps", "support_eip", v))
	}
	if d.HasChange("max_tps") {
		update = true
	}
	if v, ok := d.GetOk("max_tps"); ok {
		parameterMapList = append(parameterMapList, map[string]interface{}{
			"Code":  "MaxTps",
			"Value": v,
		})
	}
	if d.HasChange("queue_capacity") {
		update = true
	}
	if v, ok := d.GetOk("queue_capacity"); ok {
		parameterMapList = append(parameterMapList, map[string]interface{}{
			"Code":  "QueueCapacity",
			"Value": v,
		})
	}

	if d.HasChange("support_eip") || d.IsNewResource() {
		update = true
	}
	if v, ok := d.GetOkExists("support_eip"); ok {
		parameterMapList = append(parameterMapList, map[string]interface{}{
			"Code":  "SupportEip",
			"Value": convertAmqpInstanceSupportEipRequest(v),
		})
	}
	modifyInstanceReq["Parameter"] = parameterMapList
	modifyInstanceReq["SubscriptionType"] = d.Get("payment_type")
	modifyInstanceReq["ProductCode"] = "ons"
	modifyInstanceReq["ProductType"] = "ons_onsproxy_pre"
	if update {
		if v, ok := d.GetOk("modify_type"); ok {
			modifyInstanceReq["ModifyType"] = v
		}
		action := "ModifyInstance"
		conn, err := client.NewBssopenapiClient()
		if err != nil {
			return WrapError(err)
		}
		request["ClientToken"] = buildClientToken("ModifyInstance")
		runtime := util.RuntimeOptions{}
		runtime.SetAutoretry(true)
		wait := incrementalWait(3*time.Second, 3*time.Second)
		err = resource.Retry(d.Timeout(schema.TimeoutUpdate), func() *resource.RetryError {
			response, err = conn.DoRequest(StringPointer(action), nil, StringPointer("POST"), StringPointer("2017-12-14"), StringPointer("AK"), nil, modifyInstanceReq, &runtime)
			if err != nil {
				if NeedRetry(err) {
					wait()
					return resource.RetryableError(err)
				}
				if IsExpectedErrors(err, []string{"NotApplicable"}) {
					conn.Endpoint = String(connectivity.BssOpenAPIEndpointInternational)
					return resource.RetryableError(err)
				}
				return resource.NonRetryableError(err)
			}
			return nil
		})
		addDebug(action, response, modifyInstanceReq)
		if err != nil {
			return WrapErrorf(err, DefaultErrorMsg, d.Id(), action, AlibabaCloudSdkGoERROR)
		}
		if fmt.Sprint(response["Code"]) != "Success" {
			return WrapError(fmt.Errorf("%s failed, response: %v", action, response))
		}
		d.SetPartial("max_eip_tps")
		d.SetPartial("max_tps")
		d.SetPartial("payment_type")
		d.SetPartial("queue_capacity")
		d.SetPartial("support_eip")
	}
	d.Partial(false)
	return resourceAlicloudAmqpInstanceRead(d, meta)
}
func resourceAlicloudAmqpInstanceDelete(d *schema.ResourceData, meta interface{}) error {
	log.Printf("[WARN] Cannot destroy resourceAlicloudAmqpInstance. Terraform will remove this resource from the state file, however resources may remain.")
	return nil
}
func convertAmqpInstanceTypeResponse(source interface{}) interface{} {
	switch source {
	case "PROFESSIONAL":
		return "professional"
	case "VIP":
		return "vip"
	}
	return source
}
func convertAmqpInstanceRenewalDurationUnitResponse(source interface{}) interface{} {
	switch source {
	case "M":
		return "Month"
	case "Y":
		return "Year"
	}
	return source
}
func convertAmqpInstanceRenewalDurationUnitRequest(source interface{}) interface{} {
	switch source {
	case "Month":
		return "M"
	case "Year":
		return "Y"
	}
	return source
}

func convertAmqpInstanceSupportEipRequest(source interface{}) interface{} {
	switch source {
	case true:
		return "eip_true"
	case false:
		return "eip_false"
	}
	return source
}
