var data = 
    [
        {
            "Origin": "nodeA",
            "ID": 1,
            "Text": "one"
        },
        {
            "Origin": "nodeB",
            "ID": 1,
            "Text": "two"
        },
        {
            "Origin": "nodeC",
            "ID": 1,
            "Text": "three"
        },
        {
            "Origin": "nodeD",
            "ID": 1,
            "Text": "four"
        },
        
    ];

$(function() {
    $('#msgTable').bootstrapTable({
        data: data
    });
});

$(function() {
    $.getJSON("http://localhost:10000/message", function(data, status){
        console.log("HAH")
        $('#msgTable').bootstrapTable({
            data: data
        });
        console.log("HUH")
    });
});
