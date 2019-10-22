
* There are some settings which are available for all http inputs. Currently we link to these configs like ssl, how will we do that in the future?
* What about shared docs?
  * Do we reuse and copy?
  * Link to the website?
  * Update on config option = update of package?
  * How to community packages use it?


* Should we call the file also `manifest.yml` or `inputs.yml`? 
    * `manifest.yml`.
    
* The fields.yml must be global
  * How do we handle autodiscover and processor fields
  * Could we skip generating keyword fields?
  
  
* Only 1 input definition possible.
* Do we need reuse to get started?
* Where do global configs like prometheus metrics go?
  * How do we know an index is a prometheus metric index? Relevant?

TODO

* Add example for light module
* Document how ingest pipeline reference works with json and yaml


## Definition of vars

```
vars:
  -
    # Name of the variable that should be replaced
    name: hosts

    # Default value of the variable which is used in the UI and in the config if not specified
    default:
      ["http://127.0.0.1"]
    required: true

    # OS specific configurations!
    os.darwin:
      - /usr/local/var/log/nginx/error.log*
    os.windows:
      - c:/programdata/nginx/logs/error.log*


    # Below are UI Configs. Should we prefix these with ui.*?

    # Title used for the UI
    title: "Hosts lists"

    # Description of the varaiable which could be used in the UI
    description: Nginx hosts

    # A special type can be specified here for the UI Input document. By default it is just a 
    # text field.
    type: password

```
