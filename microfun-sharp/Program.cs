using System;

class Program
{
    static int Main(string[] args)
    {
        // Get argument from command line

        if (args.Length < 1)
        {
            Console.WriteLine("Invalid arguments");
            PrintUsage();
            return 1;
        }

        var path = args[0];

        ParseFile("prelude.mf");
        ParseFile(path);

        Console.WriteLine("Done");

        return 0;
    }

    private static void ParseFile(string path)
    {
        // Load file

        var file = new SourceFile(path);
        var success = file.Load();

        if (!success)
        {
            Console.WriteLine("Could not load file " + path);
            return;
        }

        // Lex file

        var lexer = new Lexer(file);
        success = lexer.Lex();

        if (!success)
        {
            lexer.Report.Print();
        }
    }

    static void PrintUsage()
    {
        Console.WriteLine("usage: microfun source.mf");
    }
}