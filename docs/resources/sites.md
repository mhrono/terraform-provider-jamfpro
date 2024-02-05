---
# generated by https://github.com/hashicorp/terraform-plugin-docs
page_title: "jamfpro_sites Resource - terraform-provider-jamfpro"
subcategory: ""
description: |-
  
---

# jamfpro_sites (Resource)





<!-- schema generated by tfplugindocs -->
## Schema

### Required

- `name` (String) The unique name of the Jamf Pro site.

### Optional

- `timeouts` (Block, Optional) (see [below for nested schema](#nestedblock--timeouts))

### Read-Only

- `id` (String) The unique identifier of the site.

<a id="nestedblock--timeouts"></a>
### Nested Schema for `timeouts`

Optional:

- `create` (String)
- `delete` (String)
- `read` (String)
- `update` (String)