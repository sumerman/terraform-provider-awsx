package awsx

import (
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/aws/aws-sdk-go/service/elasticache"

	"github.com/hashicorp/terraform/helper/resource"
	"github.com/hashicorp/terraform/helper/schema"
	"github.com/hashicorp/terraform/terraform"
)

func Provider() terraform.ResourceProvider {
	return &schema.Provider{
		Schema: map[string]*schema.Schema{
			"access_key": &schema.Schema{
				Type:        schema.TypeString,
				Optional:    true,
				Default:     "",
				Description: descriptions["access_key"],
			},

			"secret_key": &schema.Schema{
				Type:        schema.TypeString,
				Optional:    true,
				Default:     "",
				Description: descriptions["secret_key"],
			},

			"profile": &schema.Schema{
				Type:        schema.TypeString,
				Optional:    true,
				Default:     "",
				Description: descriptions["profile"],
			},

			"shared_credentials_file": &schema.Schema{
				Type:        schema.TypeString,
				Optional:    true,
				Default:     "",
				Description: descriptions["shared_credentials_file"],
			},

			"token": &schema.Schema{
				Type:        schema.TypeString,
				Optional:    true,
				Default:     "",
				Description: descriptions["token"],
			},

			"region": &schema.Schema{
				Type:     schema.TypeString,
				Required: true,
				DefaultFunc: schema.MultiEnvDefaultFunc([]string{
					"AWS_REGION",
					"AWS_DEFAULT_REGION",
				}, nil),
				Description:  descriptions["region"],
				InputDefault: "us-east-1",
			},

			"max_retries": &schema.Schema{
				Type:        schema.TypeInt,
				Optional:    true,
				Default:     11,
				Description: descriptions["max_retries"],
			},
		},

		ResourcesMap: map[string]*schema.Resource{
			"awsx_elasticache_replication_group": resourceAwsElasticacheReplicationGroup(),
		},
		ConfigureFunc: providerConfigure,
	}
}

func resourceAwsElasticacheReplicationGroup() *schema.Resource {
	return &schema.Resource{
		Create: resourceAwsElasticacheReplictaionGroupCreate,
		Read:   resourceAwsElasticacheReplictaionGroupRead,
		//Update: resourceAwsElasticacheReplictaionGroupUpdate, TODO
		Delete: resourceAwsElasticacheReplictaionGroupDelete,

		Schema: map[string]*schema.Schema{
			// TODO cluster_id
			"replication_group_id": &schema.Schema{
				Type:     schema.TypeString,
				Required: true,
				ForceNew: true,
				StateFunc: func(val interface{}) string {
					// Elasticache normalizes cluster ids to lowercase,
					// so we have to do this too or else we can end up
					// with non-converging diffs.
					return strings.ToLower(val.(string))
				},
				ValidateFunc: validateElastiCacheReplictionGroupId,
			},
			"node_type": &schema.Schema{
				Type:     schema.TypeString,
				Required: true,
				ForceNew: true,
			},
			"num_cache_clusters": &schema.Schema{
				Type:     schema.TypeInt,
				Required: true,
				ForceNew: true, // TODO add update
			},
			"parameter_group_name": &schema.Schema{
				Type:     schema.TypeString,
				Optional: true,
				Computed: true,
			},
			"port": &schema.Schema{
				Type:     schema.TypeInt,
				Required: true,
				ForceNew: true,
			},
			"engine_version": &schema.Schema{
				Type:     schema.TypeString,
				Optional: true,
				Computed: true,
			},
			"maintenance_window": &schema.Schema{
				Type:     schema.TypeString,
				Optional: true,
				Computed: true,
				StateFunc: func(val interface{}) string {
					// Elasticache always changes the maintenance
					// to lowercase
					return strings.ToLower(val.(string))
				},
			},
			"subnet_group_name": &schema.Schema{
				Type:     schema.TypeString,
				Optional: true,
				Computed: true,
				ForceNew: true,
			},
			"security_group_names": &schema.Schema{
				Type:     schema.TypeSet,
				Optional: true,
				Computed: true,
				ForceNew: true,
				Elem:     &schema.Schema{Type: schema.TypeString},
				Set:      schema.HashString,
			},
			"security_group_ids": &schema.Schema{
				Type:     schema.TypeSet,
				Optional: true,
				Computed: true,
				Elem:     &schema.Schema{Type: schema.TypeString},
				Set:      schema.HashString,
			},
			// Exported Attributes
			"endpoint": &schema.Schema{
				Type:     schema.TypeMap,
				Computed: true,
				Elem: &schema.Resource{
					Schema: map[string]*schema.Schema{
						"address": &schema.Schema{
							Type:     schema.TypeString,
							Computed: true,
						},
						"port": &schema.Schema{
							Type:     schema.TypeInt,
							Computed: true,
						},
					},
				},
			},
			"cache_nodes": &schema.Schema{
				Type:     schema.TypeList,
				Computed: true,
				Elem: &schema.Resource{
					Schema: map[string]*schema.Schema{
						"id": &schema.Schema{
							Type:     schema.TypeString,
							Computed: true,
						},
						"address": &schema.Schema{
							Type:     schema.TypeString,
							Computed: true,
						},
						"port": &schema.Schema{
							Type:     schema.TypeInt,
							Computed: true,
						},
						"availability_zone": &schema.Schema{
							Type:     schema.TypeString,
							Computed: true,
						},
					},
				},
			},
			// A single-element string list containing an Amazon Resource Name (ARN) that
			// uniquely identifies a Redis RDB snapshot file stored in Amazon S3. The snapshot
			// file will be used to populate the node group.
			"snapshot_arns": &schema.Schema{
				Type:     schema.TypeSet,
				Optional: true,
				ForceNew: true,
				Elem:     &schema.Schema{Type: schema.TypeString},
				Set:      schema.HashString,
			},

			"snapshot_window": &schema.Schema{
				Type:     schema.TypeString,
				Optional: true,
				Computed: true,
			},

			"snapshot_retention_limit": &schema.Schema{
				Type:     schema.TypeInt,
				Optional: true,
				ForceNew: true, // TODO add update
				ValidateFunc: func(v interface{}, k string) (ws []string, es []error) {
					value := v.(int)
					if value > 35 {
						es = append(es, fmt.Errorf(
							"snapshot retention limit cannot be more than 35 days"))
					}
					return
				},
			},

			"automatic_failover": &schema.Schema{
				Type:     schema.TypeString,
				Optional: true,
				Computed: true,
				ForceNew: true,
				ValidateFunc: func(v interface{}, k string) (ws []string, es []error) {
					value := v.(string)
					if !(value == elasticache.AutomaticFailoverStatusEnabled || value == elasticache.AutomaticFailoverStatusDisabled) {
						es = append(es, fmt.Errorf("valid values for 'automatic_failover' are 'enabled' and 'disabled'"))
					}
					return
				},
			},

			"availability_zones": &schema.Schema{
				Type:     schema.TypeSet,
				Optional: true,
				ForceNew: true,
				Elem:     &schema.Schema{Type: schema.TypeString},
				Set:      schema.HashString,
			},

			//"tags": tagsSchema(), TODO

			"apply_immediately": &schema.Schema{
				Type:     schema.TypeBool,
				Optional: true,
				Computed: true,
			},
		},
	}
}

var descriptions map[string]string

func init() {
	descriptions = map[string]string{
		"region": "The region where AWS operations will take place. Examples\n" +
			"are us-east-1, us-west-2, etc.",

		"access_key": "The access key for API operations. You can retrieve this\n" +
			"from the 'Security & Credentials' section of the AWS console.",

		"secret_key": "The secret key for API operations. You can retrieve this\n" +
			"from the 'Security & Credentials' section of the AWS console.",

		"profile": "The profile for API operations. If not set, the default profile\n" +
			"created with `aws configure` will be used.",

		"shared_credentials_file": "The path to the shared credentials file. If not set\n" +
			"this defaults to ~/.aws/credentials.",

		"token": "session token. A session token is only required if you are\n" +
			"using temporary security credentials.",

		"max_retries": "The maximum number of times an AWS API request is\n" +
			"being executed. If the API request still fails, an error is\n" +
			"thrown.",
	}
}

func resourceAwsElasticacheReplictaionGroupCreate(d *schema.ResourceData, meta interface{}) error {
	conn := meta.(*elasticache.ElastiCache)

	replicationGroupId := d.Get("replication_group_id").(string)
	nodeType := d.Get("node_type").(string) // e.g) cache.m1.small
	// TODO either cluster_id or num_cache_clusters > 1
	numNodes := int64(d.Get("num_cache_clusters").(int)) // 2
	engineVersion := d.Get("engine_version").(string)    // 1.4.14
	port := int64(d.Get("port").(int))                   // e.g) 11211
	subnetGroupName := d.Get("subnet_group_name").(string)
	securityNameSet := d.Get("security_group_names").(*schema.Set)
	securityIdSet := d.Get("security_group_ids").(*schema.Set)

	securityNames := expandStringList(securityNameSet.List())
	securityIds := expandStringList(securityIdSet.List())

	req := &elasticache.CreateReplicationGroupInput{
		ReplicationGroupId:          aws.String(replicationGroupId),
		ReplicationGroupDescription: aws.String(""), // TODO?
		CacheNodeType:               aws.String(nodeType),
		NumCacheClusters:            aws.Int64(numNodes),
		Engine:                      aws.String("redis"),
		EngineVersion:               aws.String(engineVersion),
		Port:                        aws.Int64(port),
		CacheSubnetGroupName:        aws.String(subnetGroupName),
		CacheSecurityGroupNames:     securityNames,
		SecurityGroupIds:            securityIds,
	}

	// parameter groups are optional and can be defaulted by AWS
	if v, ok := d.GetOk("parameter_group_name"); ok {
		req.CacheParameterGroupName = aws.String(v.(string))
	}

	if v, ok := d.GetOk("snapshot_retention_limit"); ok {
		req.SnapshotRetentionLimit = aws.Int64(int64(v.(int)))
	}

	if v, ok := d.GetOk("snapshot_window"); ok {
		req.SnapshotWindow = aws.String(v.(string))
	}

	if v, ok := d.GetOk("maintenance_window"); ok {
		req.PreferredMaintenanceWindow = aws.String(v.(string))
	}

	snaps := d.Get("snapshot_arns").(*schema.Set).List()
	if len(snaps) > 0 {
		s := expandStringList(snaps)
		req.SnapshotArns = s
		log.Printf("[DEBUG] Restoring Redis cluster from S3 snapshot: %#v", s)
	}

	if v, ok := d.GetOk("automatic_failover"); ok {
		req.AutomaticFailoverEnabled = aws.Bool(v.(string) == elasticache.AutomaticFailoverStatusEnabled)
	}

	preferred_azs := d.Get("availability_zones").(*schema.Set).List()
	if len(preferred_azs) > 0 {
		azs := expandStringList(preferred_azs)
		req.PreferredCacheClusterAZs = azs
	}

	resp, err := conn.CreateReplicationGroup(req)
	if err != nil {
		return fmt.Errorf("Error creating Elasticache: %s", err)
	}

	// Assign the cluster id as the resource ID
	// Elasticache always retains the id in lower case, so we have to
	// mimic that or else we won't be able to refresh a resource whose
	// name contained uppercase characters.
	d.SetId(strings.ToLower(*resp.ReplicationGroup.ReplicationGroupId))

	pending := []string{"creating"}
	stateConf := &resource.StateChangeConf{
		Pending:    pending,
		Target:     []string{"available"},
		Refresh:    replicationGroupStateRefreshFunc(conn, d.Id(), "available", pending),
		Timeout:    20 * time.Minute,
		Delay:      10 * time.Second,
		MinTimeout: 3 * time.Second,
	}

	log.Printf("[DEBUG] Waiting for state to become available: %v", d.Id())
	_, sterr := stateConf.WaitForState()
	if sterr != nil {
		return fmt.Errorf("Error waiting for elasticache (%s) to be created: %s", d.Id(), sterr)
	}

	return resourceAwsElasticacheReplictaionGroupRead(d, meta)
}

func resourceAwsElasticacheReplictaionGroupRead(d *schema.ResourceData, meta interface{}) error {
	conn := meta.(*elasticache.ElastiCache)
	req := &elasticache.DescribeReplicationGroupsInput{
		ReplicationGroupId: aws.String(d.Id()),
	}

	res, err := conn.DescribeReplicationGroups(req)
	if err != nil {
		if eccErr, ok := err.(awserr.Error); ok && eccErr.Code() == "ReplicationGroupNotFoundFault" {
			log.Printf("[WARN] ElastiCache Replication group (%s) not found", d.Id())
			d.SetId("")
			return nil
		}

		return err
	}

	if len(res.ReplicationGroups) == 1 {
		c := res.ReplicationGroups[0]
		d.Set("replication_group_id", c.ReplicationGroupId)
		d.Set("automatic_failover", c.AutomaticFailover)

		if len(c.NodeGroups) == 1 && len(c.NodeGroups[0].NodeGroupMembers) > 0 {
			cacheNodeData := make([]map[string]interface{}, 0, len(c.NodeGroups[0].NodeGroupMembers))
			for _, node := range c.NodeGroups[0].NodeGroupMembers {
				cacheNodeData = append(cacheNodeData, map[string]interface{}{
					"id":                *node.CacheClusterId,
					"address":           *node.ReadEndpoint.Address,
					"port":              int(*node.ReadEndpoint.Port),
					"availability_zone": *node.PreferredAvailabilityZone,
				})
			}
			d.Set("cache_nodes", cacheNodeData)

			d.Set("endpoint", map[string]interface{}{
				"address": *c.NodeGroups[0].PrimaryEndpoint.Address,
				"port":    int(*c.NodeGroups[0].PrimaryEndpoint.Port),
			})

			n := c.NodeGroups[0].NodeGroupMembers[0]
			req := &elasticache.DescribeCacheClustersInput{
				CacheClusterId:    n.CacheClusterId,
				ShowCacheNodeInfo: aws.Bool(true),
			}

			res, err := conn.DescribeCacheClusters(req)
			if err != nil {
				return err
			}
			if len(res.CacheClusters) == 1 {
				c := res.CacheClusters[0]
				d.Set("node_type", c.CacheNodeType)
				d.Set("num_cache_nodes", c.NumCacheNodes)
				d.Set("engine", c.Engine)
				d.Set("engine_version", c.EngineVersion)
				d.Set("subnet_group_name", c.CacheSubnetGroupName)
				d.Set("security_group_names", c.CacheSecurityGroups)
				d.Set("security_group_ids", c.SecurityGroups)
				d.Set("parameter_group_name", c.CacheParameterGroup)
				d.Set("maintenance_window", c.PreferredMaintenanceWindow)
				d.Set("snapshot_window", c.SnapshotWindow)
				d.Set("snapshot_retention_limit", c.SnapshotRetentionLimit)
			}
		}
	}

	return nil
}

func resourceAwsElasticacheReplictaionGroupDelete(d *schema.ResourceData, meta interface{}) error {
	conn := meta.(*elasticache.ElastiCache)

	req := &elasticache.DeleteReplicationGroupInput{
		ReplicationGroupId: aws.String(d.Id()),
		// TODO retain primary?
	}
	if _, err := conn.DeleteReplicationGroup(req); err != nil {
		return err
	}

	log.Printf("[DEBUG] Waiting for deletion: %v", d.Id())
	stateConf := &resource.StateChangeConf{
		Pending:    []string{"creating", "available", "deleting", "incompatible-parameters", "incompatible-network", "restore-failed"},
		Target:     []string{},
		Refresh:    replicationGroupStateRefreshFunc(conn, d.Id(), "", []string{}),
		Timeout:    20 * time.Minute,
		Delay:      10 * time.Second,
		MinTimeout: 3 * time.Second,
	}

	_, sterr := stateConf.WaitForState()
	if sterr != nil {
		return fmt.Errorf("Error waiting for elasticache (%s) to delete: %s", d.Id(), sterr)
	}

	d.SetId("")

	return nil
}

func replicationGroupStateRefreshFunc(conn *elasticache.ElastiCache, replGroupID, givenState string, pending []string) resource.StateRefreshFunc {
	return func() (interface{}, string, error) {
		resp, err := conn.DescribeReplicationGroups(&elasticache.DescribeReplicationGroupsInput{
			ReplicationGroupId: aws.String(replGroupID),
		})
		if err != nil {
			apierr := err.(awserr.Error)
			log.Printf("[DEBUG] message: %v, code: %v", apierr.Message(), apierr.Code())
			if apierr.Message() == fmt.Sprintf("ReplicationGroup %v not found.", replGroupID) {
				log.Printf("[DEBUG] Detect deletion")
				return nil, "", nil
			}

			log.Printf("[ERROR] ReplicationGroupStateRefreshFunc: %s", err)
			return nil, "", err
		}

		if len(resp.ReplicationGroups) == 0 {
			return nil, "", fmt.Errorf("[WARN] Error: no Replication Groups found for id (%s)", replGroupID)
		}

		var rg *elasticache.ReplicationGroup
		for _, group := range resp.ReplicationGroups {
			if *group.ReplicationGroupId == replGroupID {
				log.Printf("[DEBUG] Found matching ElastiCache replication group: %s", *group.ReplicationGroupId)
				rg = group
			}
		}

		if rg == nil {
			return nil, "", fmt.Errorf("[WARN] Error: no matching Elastic Cache replication group for id (%s)", replGroupID)
		}

		log.Printf("[DEBUG] ElastiCache Replication Group (%s) status: %v", replGroupID, *rg.Status)

		// return the current state if it's in the pending array
		for _, p := range pending {
			s := *rg.Status
			log.Printf("[DEBUG] ElastiCache: checking pending state (%s) for replication group (%s), group status: %s", pending, replGroupID, s)
			if p == s {
				log.Printf("[DEBUG] Return with status: %v", s)
				return rg, p, nil
			}
		}

		// return given state if it's not in pending
		if givenState != "" {
			log.Printf("[DEBUG] ElastiCache: checking given state (%s) of a replication group (%s) against group status (%s)", givenState, replGroupID, *rg.ReplicationGroupId)

			// loop the nodes and check their status as well
			if len(rg.NodeGroups) == 1 {
				status := rg.NodeGroups[0].Status
				if status != nil && *status != "available" {
					log.Printf("[DEBUG] Node group (%s) is not yet available, status: %s", *rg.NodeGroups[0].NodeGroupId, *status)
					return nil, "creating", nil
				}
				log.Printf("[DEBUG] Cache node group is not in an expected state")
			}

			log.Printf("[DEBUG] ElastiCache returning given state (%s), replication group: %s", givenState, rg)
			return rg, givenState, nil
		}

		log.Printf("[DEBUG] current status: %v", *rg.Status)
		return rg, *rg.Status, nil
	}
}
