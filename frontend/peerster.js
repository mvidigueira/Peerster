var peerCount = 0;

$(document).ready(function() {
    $('#privateMsgForm').on('submit', function(e) {
        e.preventDefault();
        var text = $("textarea#privateMessageText").val();
        var destination = $('select#pmDestination').val()
        $.ajax({
            type: $(this).attr('method'),
            url: $(this).attr('action'),
            data: {message:text, destName:destination},
        });
    });

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

    $('#fileSharingForm').on('submit', function(e) {
        e.preventDefault();
        var fileName = $("input#shareFile").val().replace(/^.*[\\\/]/, '');

        $.ajax({
            type: $(this).attr('method'),
            url: $(this).attr('action'),
            data: {file:fileName},
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

    $.getJSON("/origins", function(d, status){
        var sel = $('#pmDestination');
        var selectedOption = $("#pmDestination option:selected").val();
        sel.empty()

        var options = (sel.prop)? sel.prop('options') : sel.attr('options');

        $.each(d,function(i, value) {
            options[options.length] = new Option(value.Name, value.Name);
        });
        sel.val(selectedOption);
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

    $.getJSON("/origins", function(d, status){
        var sel = $('#pmDestination');
        var selectedOption = $("#pmDestination option:selected").val();
        sel.empty()

        var options = (sel.prop)? sel.prop('options') : sel.attr('options');

        $.each(d,function(i, value) {
            options[options.length] = new Option(value.Name, value.Name);
        });
        sel.val(selectedOption);

        //$('#pmDestination').html("Peerster Client User Interface - " + d);
    });
}, 1000);