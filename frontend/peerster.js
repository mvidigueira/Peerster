var peerCount = 0;

$(document).ready(function() {
    $('#msgForm').on('submit', function(e) {
        e.preventDefault();
        var text = $("textarea#messageText").val();

        $.ajax({
            type: $(this).attr('method'),
            url: $(this).attr('action'),
            data: {message:text},
        });
    });

    $('#nodeForm').on('submit', function(e) {
        e.preventDefault();
        var addr = $("input#peerAdress").val();

        $.ajax({
            type: $(this).attr('method'),
            url: $(this).attr('action'),
            data: {peer:addr},
        });

    });

    loadTables()
});

function loadTables() {
    $.getJSON("/message", function(d, status) {
        $('#msgTable').bootstrapTable({
            data: d
        });
    });

    $.getJSON("/node", function(d, status){
        $('#peerTable').bootstrapTable({
            data: d
        });
        peerCount = d.length
    });

    $.getJSON("/id", function(d, status){
        $('#nodeName').html("Peerster Client User Interface - " + d);
    });
}


window.setInterval(function() {
    $.getJSON("/message", function(d, status){
        if(d.length > 0) {
            $('#msgTable').bootstrapTable("append", d)
        }
    });

    $.getJSON("/node", function(d, status){
        if(d.length > peerCount) {
            peerCount = d.length
            $('#peerTable').bootstrapTable("refresh", d)
        }
    });
}, 1000);