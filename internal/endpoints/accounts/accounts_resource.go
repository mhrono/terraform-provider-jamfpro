// accounts_resource.go
package accounts

import (
	"context"
	"fmt"
	"log"
	"strconv"
	"time"

	"github.com/deploymenttheory/go-api-sdk-jamfpro/sdk/http_client"
	"github.com/deploymenttheory/go-api-sdk-jamfpro/sdk/jamfpro"
	"github.com/deploymenttheory/terraform-provider-jamfpro/internal/client"
	"github.com/deploymenttheory/terraform-provider-jamfpro/internal/endpoints/common"
	util "github.com/deploymenttheory/terraform-provider-jamfpro/internal/helpers/type_assertion"

	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/retry"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
)

// ResourceJamfProAccount defines the schema and CRUD operations for managing buildings in Terraform.
func ResourceJamfProAccounts() *schema.Resource {
	return &schema.Resource{
		CreateContext: ResourceJamfProAccountCreate,
		ReadContext:   ResourceJamfProAccountRead,
		UpdateContext: ResourceJamfProAccountUpdate,
		DeleteContext: ResourceJamfProAccountDelete,
		CustomizeDiff: customDiffAccounts,
		Timeouts: &schema.ResourceTimeout{
			Create: schema.DefaultTimeout(1 * time.Minute),
			Read:   schema.DefaultTimeout(1 * time.Minute),
			Update: schema.DefaultTimeout(1 * time.Minute),
			Delete: schema.DefaultTimeout(1 * time.Minute),
		},
		Schema: map[string]*schema.Schema{
			"id": {
				Type:        schema.TypeString,
				Computed:    true,
				Description: "The unique identifier of the jamf pro account.",
			},
			"name": {
				Type:        schema.TypeString,
				Required:    true,
				Description: "The name of the jamf pro account.",
			},
			"directory_user": {
				Type:        schema.TypeBool,
				Optional:    true,
				Description: "Indicates if the user is a directory user.",
			},
			"full_name": {
				Type:        schema.TypeString,
				Optional:    true,
				Description: "The full name of the account user.",
			},
			"email": {
				Type:        schema.TypeString,
				Optional:    true,
				Description: "The email of the account user.",
			},
			"email_address": {
				Type:        schema.TypeString,
				Optional:    true,
				Description: "The email address of the account user.",
			},
			"enabled": {
				Type:        schema.TypeString,
				Required:    true,
				Description: "Access status of the account (“enabled” or “disabled”).",
				ValidateFunc: func(val interface{}, key string) (warns []string, errs []error) {
					v := util.GetString(val)
					if v == "Enabled" || v == "Disabled" {
						return
					}
					errs = append(errs, fmt.Errorf("%q must be either 'Enabled' or 'Disabled', got: %s", key, v))
					return warns, errs
				},
			},
			"ldap_server": {
				Type:        schema.TypeList,
				Optional:    true,
				MaxItems:    1,
				Description: "LDAP server information associated with the account.",
				Elem: &schema.Resource{
					Schema: map[string]*schema.Schema{
						"id": {
							Type:        schema.TypeInt,
							Optional:    true,
							Description: "The ID of the LDAP server.",
							Default:     "",
						},
						"name": {
							Type:        schema.TypeString,
							Optional:    true,
							Description: "The name of the LDAP server.",
							Computed:    true,
						},
					},
				},
			},
			"force_password_change": {
				Type:        schema.TypeBool,
				Optional:    true,
				Description: "Indicates if the user is forced to change password on next login.",
			},
			"access_level": {
				Type:        schema.TypeString,
				Required:    true,
				Description: "The access level of the account. This can be either Full Access, scoped to a jamf pro site with Site Access, or scoped to a jamf pro account group with Group Access",
				ValidateFunc: func(val interface{}, key string) (warns []string, errs []error) {
					v := util.GetString(val)
					if v == "Full Access" || v == "Site Access" || v == "Group Access" {
						return
					}
					errs = append(errs, fmt.Errorf("%q must be either 'Full Access' or 'Site Access' or 'Group Access', got: %s", key, v))
					return warns, errs
				},
			},
			"password": {
				Type:        schema.TypeString,
				Optional:    true,
				Description: "The password for the account.",
				Sensitive:   true,
			},
			"privilege_set": {
				Type:        schema.TypeString,
				Optional:    true,
				Description: "The privilege set assigned to the account.",
				ValidateFunc: func(val interface{}, key string) (warns []string, errs []error) {
					v := util.GetString(val)
					validPrivileges := []string{"Administrator", "Auditor", "Enrollment Only", "Custom"}
					for _, validPriv := range validPrivileges {
						if v == validPriv {
							return // Valid value found, return without error
						}
					}
					errs = append(errs, fmt.Errorf("%q must be one of %v, got: %s", key, validPrivileges, v))
					return warns, errs
				},
			},
			"site": {
				Type:        schema.TypeList,
				Optional:    true,
				MaxItems:    1,
				Description: "The site information associated with the account group if access_level is set to Site Access.",
				Elem: &schema.Resource{
					Schema: map[string]*schema.Schema{
						"id": {
							Type:        schema.TypeInt,
							Optional:    true,
							Description: "Jamf Pro Site ID. Value defaults to '0' aka not used.",
							Default:     "",
						},
						"name": {
							Type:        schema.TypeString,
							Optional:    true,
							Description: "Jamf Pro Site Name",
							Computed:    true,
						},
					},
				},
			},
			"groups": {
				Type:        schema.TypeSet,
				Optional:    true,
				Description: "A set of group names and IDs associated with the account.",
				Elem: &schema.Resource{
					Schema: map[string]*schema.Schema{
						"name": {
							Type:     schema.TypeString,
							Required: true,
						},
						"id": {
							Type:     schema.TypeInt,
							Computed: true,
						},
						"site": {
							Type:     schema.TypeList,
							Optional: true,
							MaxItems: 1,
							Elem: &schema.Resource{
								Schema: map[string]*schema.Schema{
									"id": {
										Type:     schema.TypeInt,
										Optional: true,
									},
									"name": {
										Type:     schema.TypeString,
										Optional: true,
										Computed: true,
									},
								},
							},
						},
						"jss_objects_privileges": {
							Type:        schema.TypeList,
							Optional:    true,
							Description: "Privileges related to JSS Objects.",
							Elem: &schema.Schema{
								Type:         schema.TypeString,
								ValidateFunc: common.ValidateJSSObjectsPrivileges,
							},
						},
						"jss_settings_privileges": {
							Type:        schema.TypeList,
							Optional:    true,
							Description: "Privileges related to JSS Settings.",
							Elem: &schema.Schema{
								Type:         schema.TypeString,
								ValidateFunc: common.ValidateJSSSettingsPrivileges,
							},
						},
						"jss_actions_privileges": {
							Type:        schema.TypeList,
							Optional:    true,
							Description: "Privileges related to JSS Actions.",
							Elem: &schema.Schema{
								Type:         schema.TypeString,
								ValidateFunc: common.ValidateJSSActionsPrivileges,
							},
						},
						"casper_admin_privileges": {
							Type:        schema.TypeList,
							Optional:    true,
							Description: "Privileges related to Casper Admin.",
							Elem: &schema.Schema{
								Type:         schema.TypeString,
								ValidateFunc: common.ValidateCasperAdminPrivileges,
							},
						},
						"casper_remote_privileges": {
							Type:        schema.TypeList,
							Optional:    true,
							Description: "Privileges related to Casper Remote.",
							Elem: &schema.Schema{
								Type: schema.TypeString,
							},
						},
						"casper_imaging_privileges": {
							Type:        schema.TypeList,
							Optional:    true,
							Description: "Privileges related to Casper Imaging.",
							Elem: &schema.Schema{
								Type: schema.TypeString,
							},
						},
						"recon_privileges": {
							Type:        schema.TypeList,
							Optional:    true,
							Description: "Privileges related to Recon.",
							Elem: &schema.Schema{
								Type: schema.TypeString,
							},
						},
					},
				},
			},
			"jss_objects_privileges": {
				Type:        schema.TypeList,
				Optional:    true,
				Description: "Privileges related to JSS Objects.",
				Elem: &schema.Schema{
					Type:         schema.TypeString,
					ValidateFunc: common.ValidateJSSObjectsPrivileges,
				},
			},
			"jss_settings_privileges": {
				Type:        schema.TypeList,
				Optional:    true,
				Description: "Privileges related to JSS Settings.",
				Elem: &schema.Schema{
					Type:         schema.TypeString,
					ValidateFunc: common.ValidateJSSSettingsPrivileges,
				},
			},
			"jss_actions_privileges": {
				Type:        schema.TypeList,
				Optional:    true,
				Description: "Privileges related to JSS Actions.",
				Elem: &schema.Schema{
					Type:         schema.TypeString,
					ValidateFunc: common.ValidateJSSActionsPrivileges,
				},
			},
			"casper_admin_privileges": {
				Type:        schema.TypeList,
				Optional:    true,
				Description: "Privileges related to Casper Admin.",
				Elem: &schema.Schema{
					Type:         schema.TypeString,
					ValidateFunc: common.ValidateCasperAdminPrivileges,
				},
			},
			"casper_remote_privileges": {
				Type:        schema.TypeList,
				Optional:    true,
				Description: "Privileges related to Casper Remote.",
				Elem: &schema.Schema{
					Type: schema.TypeString,
				},
			},
			"casper_imaging_privileges": {
				Type:        schema.TypeList,
				Optional:    true,
				Description: "Privileges related to Casper Imaging.",
				Elem: &schema.Schema{
					Type: schema.TypeString,
				},
			},
			"recon_privileges": {
				Type:        schema.TypeList,
				Optional:    true,
				Description: "Privileges related to Recon.",
				Elem: &schema.Schema{
					Type: schema.TypeString,
				},
			},
		},
	}
}

// constructJamfProAccount constructs an Account object from the provided schema data.
func constructJamfProAccount(d *schema.ResourceData, client *jamfpro.Client) (*jamfpro.ResourceAccount, error) {
	//func constructJamfProAccount(d *schema.ResourceData) (*jamfpro.ResourceAccount, error) {
	account := &jamfpro.ResourceAccount{}

	// Utilize type assertion helper functions for direct field extraction
	account.Name = util.GetStringFromInterface(d.Get("name"))
	account.DirectoryUser = d.Get("directory_user").(bool)
	account.FullName = util.GetStringFromInterface(d.Get("full_name"))
	account.Email = util.GetStringFromInterface(d.Get("email"))
	account.Enabled = util.GetStringFromInterface(d.Get("enabled"))
	account.ForcePasswordChange = d.Get("force_password_change").(bool)
	account.AccessLevel = util.GetStringFromInterface(d.Get("access_level"))
	account.Password = util.GetStringFromInterface(d.Get("password"))
	account.PrivilegeSet = util.GetStringFromInterface(d.Get("privilege_set"))

	// Construct LDAP Server
	if v, ok := d.GetOk("ldap_server"); ok {
		ldapServerList := v.([]interface{})
		if len(ldapServerList) > 0 && ldapServerList[0] != nil {
			ldapServerMap := ldapServerList[0].(map[string]interface{})
			account.LdapServer = jamfpro.AccountSubsetLdapServer{
				ID:   util.GetIntFromInterface(ldapServerMap["id"]),
				Name: util.GetStringFromInterface(ldapServerMap["name"]),
			}
		}
	}

	// Construct Site
	if v, ok := d.GetOk("site"); ok {
		siteList := v.([]interface{})
		if len(siteList) > 0 && siteList[0] != nil {
			siteMap := siteList[0].(map[string]interface{})
			account.Site = jamfpro.SharedResourceSite{
				ID:   util.GetIntFromInterface(siteMap["id"]),
				Name: util.GetStringFromInterface(siteMap["name"]),
			}
		}
	}

	// Get all accounts to map group names to IDs
	allAccounts, err := client.GetAccounts()
	if err != nil {
		return nil, err
	}

	groupNameToID := make(map[string]int)
	for _, group := range allAccounts.Groups {
		groupNameToID[group.Name] = group.ID
	}

	// Construct Groups with inferred IDs
	if v, ok := d.GetOk("groups"); ok {
		groupsSet := v.(*schema.Set)
		for _, groupItem := range groupsSet.List() {
			groupMap := groupItem.(map[string]interface{})
			groupName := groupMap["name"].(string)
			groupID, exists := groupNameToID[groupName]
			if !exists {
				return nil, fmt.Errorf("group name %s does not exist", groupName)
			}
			group := jamfpro.AccountsListSubsetGroups{
				ID:   groupID,
				Name: groupName,
			}
			// ... Process other fields like 'site' and 'privileges' similarly
			account.Groups = append(account.Groups, group)
		}
	}

	// Construct Privileges
	account.Privileges = jamfpro.AccountSubsetPrivileges{
		JSSObjects:    util.GetStringSliceFromInterface(d.Get("jss_objects_privileges")),
		JSSSettings:   util.GetStringSliceFromInterface(d.Get("jss_settings_privileges")),
		JSSActions:    util.GetStringSliceFromInterface(d.Get("jss_actions_privileges")),
		CasperAdmin:   util.GetStringSliceFromInterface(d.Get("casper_admin_privileges")),
		CasperRemote:  util.GetStringSliceFromInterface(d.Get("casper_remote_privileges")),
		CasperImaging: util.GetStringSliceFromInterface(d.Get("casper_imaging_privileges")),
		Recon:         util.GetStringSliceFromInterface(d.Get("recon_privileges")),
	}

	// Log the successful construction of the account
	log.Printf("[INFO] Successfully constructed Account with name: %s", account.Name)

	return account, nil
}

// Helper function to generate diagnostics based on the error type.
func generateTFDiagsFromHTTPError(err error, d *schema.ResourceData, action string) diag.Diagnostics {
	var diags diag.Diagnostics
	resourceName, exists := d.GetOk("name")
	if !exists {
		resourceName = "unknown"
	}

	// Handle the APIError in the diagnostic
	if apiErr, ok := err.(*http_client.APIError); ok {
		diags = append(diags, diag.Diagnostic{
			Severity: diag.Error,
			Summary:  fmt.Sprintf("Failed to %s the resource with name: %s", action, resourceName),
			Detail:   fmt.Sprintf("API Error (Code: %d): %s", apiErr.StatusCode, apiErr.Message),
		})
	} else {
		diags = append(diags, diag.Diagnostic{
			Severity: diag.Error,
			Summary:  fmt.Sprintf("Failed to %s the resource with name: %s", action, resourceName),
			Detail:   err.Error(),
		})
	}
	return diags
}

// ResourceJamfProAccountCreate is responsible for creating a new Jamf Pro Script in the remote system.
// The function:
// 1. Constructs the attribute data using the provided Terraform configuration.
// 2. Calls the API to create the attribute in Jamf Pro.
// 3. Updates the Terraform state with the ID of the newly created attribute.
// 4. Initiates a read operation to synchronize the Terraform state with the actual state in Jamf Pro.
func ResourceJamfProAccountCreate(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	var diags diag.Diagnostics

	// Asserts 'meta' as '*client.APIClient'
	apiclient, ok := meta.(*client.APIClient)
	if !ok {
		return diag.Errorf("error asserting meta as *client.APIClient")
	}

	// The apiclient, which is of type *client.APIClient, holds a reference to the Jamf Pro client in its Conn field.
	// By assigning apiclient.Conn to jamfProClient, we are obtaining the actual Jamf Pro client (*jamfpro.Client)
	// that will be used for making API calls to the Jamf Pro server.
	// This Jamf Pro client is then passed to other functions that require it, like constructJamfProAccount.
	jamfProClient := apiclient.Conn

	// Use the retry function for the create operation
	var createdAccount *jamfpro.ResponseAccountCreatedAndUpdated
	var err error
	err = retry.RetryContext(ctx, d.Timeout(schema.TimeoutCreate), func() *retry.RetryError {
		// Construct the account
		account, err := constructJamfProAccount(d, jamfProClient)
		if err != nil {
			return retry.NonRetryableError(fmt.Errorf("failed to construct the account for terraform create: %w", err))
		}

		// Directly call the API to create the resource
		createdAccount, err = jamfProClient.CreateAccount(account)
		if err != nil {
			// Check if the error is an APIError
			if apiErr, ok := err.(*http_client.APIError); ok {
				return retry.NonRetryableError(fmt.Errorf("API Error (Code: %d): %s", apiErr.StatusCode, apiErr.Message))
			}
			// For simplicity, we're considering all other errors as retryable
			return retry.RetryableError(err)
		}

		return nil
	})

	if err != nil {
		// If there's an error while creating the resource, generate diagnostics using the helper function.
		return generateTFDiagsFromHTTPError(err, d, "create")
	}

	// Set the ID of the created resource in the Terraform state
	d.SetId(strconv.Itoa(createdAccount.ID))

	// Use the retry function for the read operation to update the Terraform state with the resource attributes
	err = retry.RetryContext(ctx, d.Timeout(schema.TimeoutRead), func() *retry.RetryError {
		readDiags := ResourceJamfProAccountRead(ctx, d, meta)
		if len(readDiags) > 0 {
			// If readDiags is not empty, it means there's an error, so we retry
			return retry.RetryableError(fmt.Errorf("failed to read the created resource"))
		}
		return nil
	})

	if err != nil {
		// If there's an error while updating the state for the resource, generate diagnostics using the helper function.
		return generateTFDiagsFromHTTPError(err, d, "update state for")
	}

	return diags
}

// ResourceJamfProAccountRead is responsible for reading the current state of a Jamf Pro Account Group Resource from the remote system.
// The function:
// 1. Fetches the attribute's current state using its ID. If it fails then obtain attribute's current state using its Name.
// 2. Updates the Terraform state with the fetched data to ensure it accurately reflects the current state in Jamf Pro.
// 3. Handles any discrepancies, such as the attribute being deleted outside of Terraform, to keep the Terraform state synchronized.
func ResourceJamfProAccountRead(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	var diags diag.Diagnostics

	// Asserts 'meta' as '*client.APIClient'
	apiclient, ok := meta.(*client.APIClient)
	if !ok {
		return diag.Errorf("error asserting meta as *client.APIClient")
	}
	conn := apiclient.Conn

	var account *jamfpro.ResourceAccount

	// Use the retry function for the read operation
	err := retry.RetryContext(ctx, d.Timeout(schema.TimeoutRead), func() *retry.RetryError {
		// Convert the ID from the Terraform state into an integer to be used for the API request
		accountID, err := strconv.Atoi(d.Id())
		if err != nil {
			return retry.NonRetryableError(fmt.Errorf("error converting id (%s) to integer: %s", d.Id(), err))
		}

		// Try fetching the account using the ID
		account, err = conn.GetAccountByID(accountID)
		if err != nil {
			// Handle the APIError
			if apiError, ok := err.(*http_client.APIError); ok {
				return retry.NonRetryableError(fmt.Errorf("API Error (Code: %d): %s", apiError.StatusCode, apiError.Message))
			}
			// If fetching by ID fails, try fetching by Name
			accountName, ok := d.Get("name").(string)
			if !ok {
				return retry.NonRetryableError(fmt.Errorf("unable to assert 'name' as a string"))
			}

			account, err = conn.GetAccountByName(accountName)
			if err != nil {
				// Handle the APIError
				if apiError, ok := err.(*http_client.APIError); ok {
					return retry.NonRetryableError(fmt.Errorf("API Error (Code: %d): %s", apiError.StatusCode, apiError.Message))
				}
				return retry.RetryableError(err)
			}
		}
		return nil
	})

	// Handle error from the retry function
	if err != nil {
		// If there's an error while reading the resource, generate diagnostics using the helper function.
		return generateTFDiagsFromHTTPError(err, d, "read")
	}

	// Update the Terraform state with account attributes
	d.Set("name", account.Name)
	d.Set("directory_user", account.DirectoryUser)
	d.Set("full_name", account.FullName)
	d.Set("email", account.Email)
	d.Set("enabled", account.Enabled)

	// Update LDAP server information
	if account.LdapServer.ID != 0 || account.LdapServer.Name != "" {
		ldapServer := make(map[string]interface{})
		ldapServer["id"] = account.LdapServer.ID
		ldapServer["name"] = account.LdapServer.Name
		d.Set("ldap_server", []interface{}{ldapServer})
	} else {
		d.Set("ldap_server", []interface{}{}) // Clear the LDAP server data if not present
	}

	d.Set("force_password_change", account.ForcePasswordChange)
	d.Set("access_level", account.AccessLevel)
	// Set password only if it's provided in the configuration
	if _, ok := d.GetOk("password"); ok {
		d.Set("password", account.Password)
	}
	d.Set("privilege_set", account.PrivilegeSet)

	// Update site information
	if account.Site.ID != 0 || account.Site.Name != "" {
		site := make(map[string]interface{})
		site["id"] = account.Site.ID
		site["name"] = account.Site.Name
		d.Set("site", []interface{}{site})
	} else {
		d.Set("site", []interface{}{}) // Clear the site data if not present
	}

	// Construct and set the groups attribute
	groups := make([]interface{}, len(account.Groups))
	for i, group := range account.Groups {
		groupMap := make(map[string]interface{})
		groupMap["name"] = group.Name
		groupMap["id"] = group.ID

		// Construct Site subfield
		if group.Site.ID != 0 || group.Site.Name != "" {
			site := make(map[string]interface{})
			site["id"] = group.Site.ID
			site["name"] = group.Site.Name
			groupMap["site"] = []interface{}{site}
		} else {
			groupMap["site"] = []interface{}{} // Clear the Site data if not present
		}

		// Map privileges from the AccountSubsetPrivileges struct to the Terraform schema
		groupMap["jss_objects_privileges"] = group.Privileges.JSSObjects
		groupMap["jss_settings_privileges"] = group.Privileges.JSSSettings
		groupMap["jss_actions_privileges"] = group.Privileges.JSSActions
		groupMap["casper_admin_privileges"] = group.Privileges.CasperAdmin
		groupMap["casper_remote_privileges"] = group.Privileges.CasperRemote
		groupMap["casper_imaging_privileges"] = group.Privileges.CasperImaging
		groupMap["recon_privileges"] = group.Privileges.Recon

		groups[i] = groupMap
	}

	if err := d.Set("groups", groups); err != nil {
		return diag.FromErr(err)
	}

	// Update privileges
	if err := d.Set("jss_objects_privileges", account.Privileges.JSSObjects); err != nil {
		return diag.FromErr(err)
	}
	if err := d.Set("jss_settings_privileges", account.Privileges.JSSSettings); err != nil {
		return diag.FromErr(err)
	}
	if err := d.Set("jss_actions_privileges", account.Privileges.JSSActions); err != nil {
		return diag.FromErr(err)
	}
	if err := d.Set("casper_admin_privileges", account.Privileges.CasperAdmin); err != nil {
		return diag.FromErr(err)
	}
	if err := d.Set("casper_remote_privileges", account.Privileges.CasperRemote); err != nil {
		return diag.FromErr(err)
	}
	if err := d.Set("casper_imaging_privileges", account.Privileges.CasperImaging); err != nil {
		return diag.FromErr(err)
	}
	if err := d.Set("recon_privileges", account.Privileges.Recon); err != nil {
		return diag.FromErr(err)
	}

	return diags
}

// ResourceJamfProAccountUpdate is responsible for updating an existing Jamf Pro Account Group on the remote system.
func ResourceJamfProAccountUpdate(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	var diags diag.Diagnostics

	// Asserts 'meta' as '*client.APIClient'
	apiclient, ok := meta.(*client.APIClient)
	if !ok {
		return diag.Errorf("error asserting meta as *client.APIClient")
	}

	// The apiclient, which is of type *client.APIClient, holds a reference to the Jamf Pro client in its Conn field.
	// By assigning apiclient.Conn to jamfProClient, we are obtaining the actual Jamf Pro client (*jamfpro.Client)
	// that will be used for making API calls to the Jamf Pro server.
	// This Jamf Pro client is then passed to other functions that require it, like constructJamfProAccount.
	jamfProClient := apiclient.Conn

	// Use the retry function for the update operation
	err := retry.RetryContext(ctx, d.Timeout(schema.TimeoutUpdate), func() *retry.RetryError {
		// Construct the updated account
		account, err := constructJamfProAccount(d, jamfProClient)
		if err != nil {
			return retry.NonRetryableError(fmt.Errorf("failed to construct the account for terraform update: %w", err))
		}

		// Obtain the ID from the Terraform state to be used for the API request
		accountID, err := strconv.Atoi(d.Id())
		if err != nil {
			return retry.NonRetryableError(fmt.Errorf("error converting id (%s) to integer: %s", d.Id(), err))
		}

		// Directly call the API to update the resource
		_, apiErr := jamfProClient.UpdateAccountByID(accountID, account)
		if apiErr != nil {
			// Handle the APIError
			if apiError, ok := apiErr.(*http_client.APIError); ok {
				return retry.NonRetryableError(fmt.Errorf("API Error (Code: %d): %s", apiError.StatusCode, apiError.Message))
			}
			// If the update by ID fails, try updating by name
			groupName, ok := d.Get("name").(string)
			if !ok {
				return retry.NonRetryableError(fmt.Errorf("unable to assert 'name' as a string in update"))
			}

			_, apiErr = jamfProClient.UpdateAccountByName(groupName, account)
			if apiErr != nil {
				// Handle the APIError
				if apiError, ok := apiErr.(*http_client.APIError); ok {
					return retry.NonRetryableError(fmt.Errorf("API Error (Code: %d): %s", apiError.StatusCode, apiError.Message))
				}
				return retry.RetryableError(apiErr)
			}
		}
		return nil
	})

	// Handle error from the retry function
	if err != nil {
		// If there's an error while updating the resource, generate diagnostics using the helper function.
		return generateTFDiagsFromHTTPError(err, d, "update")
	}

	// Use the retry function for the read operation to update the Terraform state
	err = retry.RetryContext(ctx, d.Timeout(schema.TimeoutRead), func() *retry.RetryError {
		readDiags := ResourceJamfProAccountRead(ctx, d, meta)
		if len(readDiags) > 0 {
			return retry.RetryableError(fmt.Errorf("failed to update the Terraform state for the updated resource"))
		}
		return nil
	})

	// Handle error from the retry function
	if err != nil {
		// If there's an error while updating the resource, generate diagnostics using the helper function.
		return generateTFDiagsFromHTTPError(err, d, "update")
	}

	return diags
}

// ResourceJamfProAccountDelete is responsible for deleting a Jamf Pro account group.
func ResourceJamfProAccountDelete(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	var diags diag.Diagnostics

	// Asserts 'meta' as '*client.APIClient'
	apiclient, ok := meta.(*client.APIClient)
	if !ok {
		return diag.Errorf("error asserting meta as *client.APIClient")
	}
	conn := apiclient.Conn

	// Use the retry function for the delete operation
	err := retry.RetryContext(ctx, d.Timeout(schema.TimeoutDelete), func() *retry.RetryError {
		// Obtain the ID from the Terraform state to be used for the API request
		accountID, convertErr := strconv.Atoi(d.Id())
		if convertErr != nil {
			return retry.NonRetryableError(fmt.Errorf("failed to parse dock item ID: %v", convertErr))
		}

		// Directly call the API to delete the resource
		apiErr := conn.DeleteAccountByID(accountID)
		if apiErr != nil {
			// If the delete by ID fails, try deleting by name
			accountName, ok := d.Get("name").(string)
			if !ok {
				return retry.NonRetryableError(fmt.Errorf("unable to assert 'name' as a string"))
			}

			apiErr = conn.DeleteAccountByName(accountName)
			if apiErr != nil {
				return retry.RetryableError(apiErr)
			}
		}
		return nil
	})

	// Handle error from the retry function
	if err != nil {
		// If there's an error while deleting the resource, generate diagnostics using the helper function.
		return generateTFDiagsFromHTTPError(err, d, "delete")
	}

	// Clear the ID from the Terraform state as the resource has been deleted
	d.SetId("")

	return diags
}