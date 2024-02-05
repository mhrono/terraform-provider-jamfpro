---
# generated by https://github.com/hashicorp/terraform-plugin-docs
page_title: "jamfpro_scripts Data Source - terraform-provider-jamfpro"
subcategory: ""
description: |-
  
---

# jamfpro_scripts (Data Source)





<!-- schema generated by tfplugindocs -->
## Schema

### Required

- `name` (String) Display name for the script.
- `script_contents` (String) Contents of the script. Must be non-compiled and in an accepted format.

### Read-Only

- `category_id` (String) The Jamf Pro unique identifier (ID) of the category.
- `category_name` (String) Name of the category to add the script to.
- `id` (String) The Jamf Pro unique identifier (ID) of the script.
- `info` (String) Information to display to the administrator when the script is run.
- `notes` (String) Notes to display about the script (e.g., who created it and when it was created).
- `os_requirements` (String) The script can only be run on computers with these operating system versions. Each version must be separated by a comma (e.g., 10.11, 15, 16.1).
- `parameter10` (String) Script parameter label 10
- `parameter11` (String) Script parameter label 11
- `parameter4` (String) Script parameter label 4
- `parameter5` (String) Script parameter label 5
- `parameter6` (String) Script parameter label 6
- `parameter7` (String) Script parameter label 7
- `parameter8` (String) Script parameter label 8
- `parameter9` (String) Script parameter label 9
- `priority` (String) Execution priority of the script (Before, After, At Reboot).