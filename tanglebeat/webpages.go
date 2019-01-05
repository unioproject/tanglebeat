package main

var indexPage = `
<!DOCTYPE html>
<html>
    <head>
        <title>tbreadzmq dashboard</title>
        <meta charset="UTF-8">
        <meta name="description" content="Page displays state of the Tanglebeat instance">
        <meta name="keywords" content="IOTA, Tangle, Tanglebeat, crypto, token, metrics">
        <meta name="author" content="lunfardo">
	</head>
	<style>
		table {
			border-collapse: collapse;
		}
		th, td {
		    font-family: "Liberation Mono", monospace;
			font-size: 12px;
			text-align: left;
			padding: 3px;
            vertical-align: top;
		}
		tr:nth-child(even){background-color: #f2f2f2}		
	</style>
	<script type="text/javascript" src="/loadjs">
	</script>
	<body onload=main()>
        <h3> IRI msg inputs (Zero MQ)</h3>
		 <table id="maintable" border="1">
			<tr> 
				<td>ZMQ host</td>  
		    	<td>avgBehindSNSec</td> 
     			<td>leaderTXPerc</td>
      			<td>leaderSNPerc</td>
      			<td>lastTXMsecAgo</td>
      			<td>lastSNMsecAgo</td>
      			<td>runningAlreadyMin</td>	
            </tr>
		</table>
		 <table border="0">
			<tr>
				<td><b>Compound IRI msg output (Nanomsg)</b></td>
				<td><b>Caches</b></td>
				<td><b>Go runtime</b></td>
            </tr>
            <tr>
              <td>
				 <table id="outputtable" border="1">
        		   <tr></tr>
         		</table>
              </td>
              <td>
				 <table id="rtt1" border="1">
        		   <tr></tr>
         		 </table>
              </td>
              <td>
				 <table id="rtt2" border="1">
        		   <tr></tr>
         		 </table>
              <td>
              </td>
	        </tr>
         </table>
	</body>
</html>
`

var loadjs = `
		function refresh(fun, millis){
			fun();
			setInterval(fun, millis);
		}
		function deleteChildren(obj){
    		while( obj.hasChildNodes() ){
        		obj.removeChild(obj.lastChild);
    		}
		}
		function populateRow(row, data, heading){
            if (heading){
                for (key in data){
                    if (key != "lastErr"){
		                el = document.createElement('td');
    	                el.innerHTML = "<b>" + key + "</b>";
	    	  			row.appendChild(el);
                    }
                }
            } else {
				if (data.running){
	                for (key in data){
                        if (key != "lastErr"){
			                el = document.createElement('td');
                            el.innerHTML = data[key];
	      					row.appendChild(el);
                        }
                	}
                } else {
        	        el = document.createElement('td');
	        	    el.innerHTML = data["uri"];
      			    row.appendChild(el);

        	        el = document.createElement('td');
	        	    el.innerHTML = data["running"];
      			    row.appendChild(el);

        	        el = document.createElement('td');
    	            el.setAttribute("colspan", Object.keys(data).length - 2)
	        	    el.innerHTML = data["lastErr"];
      			    row.appendChild(el);
                }
            }
        }
		function populateRoutineStats(resp){
   			tb = document.getElementById("maintable").tBodies[0];
            deleteChildren(tb);
            first = true
			for (idx in resp){
                if (first){
 			        row = document.createElement('tr');
	                populateRow(row, resp[idx], true)
		            tb.appendChild(row);
                    first = false
                }
    		    row = document.createElement('tr');
	            populateRow(row, resp[idx], false)
                row.appendChild(el);
	            tb.appendChild(row);
            }
		}
		function populate(tbname, datalist){
   			tb = document.getElementById(tbname).tBodies[0];
            deleteChildren(tb);
            for (key in datalist[0]){
                row = document.createElement('tr');

                cell = document.createElement('td');
                cell.innerHTML = key;
                row.appendChild(cell);
				for (idx in datalist){
	                cell = document.createElement('td');
    	            cell.innerHTML = datalist[idx][key];
        	        row.appendChild(cell);
            	    tb.appendChild(row);
                }
			}
		}

	    function refreshStats(){
    		var xhttp = new XMLHttpRequest();
			xhttp.onreadystatechange = function() {
        		var resp;
        		if (this.readyState == 4){
            		if (this.status == 200) {
		               resp = JSON.parse(this.response);
						populateRoutineStats(resp.zmqInputStats);
  			            populate("outputtable", [resp.zmqOutputStats, resp.zmqOutputStats10min]);
  			            populate("rtt1", [resp.zmqRuntimeStats]);
  			            populate("rtt2", [resp.goRuntimeStats]);
                    }
                }
      	    };
      	    req = "stats";
            xhttp.open("GET", req, true);
            xhttp.send();
        }

		function main(){
			refresh(refreshStats, 3000);
		}
`
