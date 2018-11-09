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

    $('#fileDownloadingForm').on('submit', function(e) {
        e.preventDefault();
        var fileName = $("textarea#dlFileName").val();
        var hash = $("textarea#dlFileHash").val();
        var from = $('select#dlFileFrom').val()

        $.ajax({
            type: $(this).attr('method'),
            url: $(this).attr('action'),
            data: {file: fileName, metahash:hash, origin: from},
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
        var sel1 = $('#pmDestination');
        var selectedOption1 = $("#pmDestination option:selected").val();
        sel1.empty()

        var options1 = (sel1.prop)? sel1.prop('options') : sel1.attr('options');

        $.each(d,function(i, value) {
            options1[options1.length] = new Option(value.Name, value.Name);
        });
        sel1.val(selectedOption1);

        var sel2 = $('#dlFileFrom');
        var selectedOption2 = $("#dlFileFrom option:selected").val();
        sel2.empty()

        var options2 = (sel2.prop)? sel2.prop('options') : sel2.attr('options');

        $.each(d,function(i, value) {
            options2[options2.length] = new Option(value.Name, value.Name);
        });
        sel2.val(selectedOption2);

        //$('#pmDestination').html("Peerster Client User Interface - " + d);
    });
}, 1000);